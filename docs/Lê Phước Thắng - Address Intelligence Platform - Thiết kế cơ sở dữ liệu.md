# Lê Phước Thắng - Address Intelligence Platform - Thiết kế cơ sở dữ liệu

> **Loại tài liệu:** Database Design Specification (DDS)  
> **Hệ thống:** Address Intelligence Platform  
> **Tác giả:** Lê Phước Thắng  
> **Trạng thái:** Design Baseline  
> **Phiên bản:** 0.3.0

## Document Control

| Thuộc tính             | Giá trị                                                                                                       |
|:-----------------------|:--------------------------------------------------------------------------------------------------------------|
| Mục đích               | Đặc tả mô hình dữ liệu canonical, dữ liệu địa lý, provenance và search read model cho Address Intelligence Platform |
| Đối tượng đọc          | Software Architect, Backend Engineer, Data Engineer, DBA, Search Engineer                                     |
| Canonical datastore    | PostgreSQL + PostGIS                                                                                          |
| Search datastore       | Elasticsearch                                                                                                 |
| Nguồn geographic chính | OpenStreetMap (OSM) + nguồn hành chính + dữ liệu nội bộ                                                       |
| Phạm vi địa lý         | Việt Nam                                                                                                      |
| Nguyên tắc nhất quán   | PostgreSQL/PostGIS là source of truth; Elasticsearch là derived read model                                    |

------------------------------------------------------------------------

## 1. Mục tiêu và phạm vi

### 1.1 Mục tiêu

Thiết kế cơ sở dữ liệu phục vụ **Address Autocomplete** cho địa chỉ Việt Nam, đồng thời tạo nền dữ liệu đủ tốt cho chuẩn hóa địa chỉ, phân giải địa chỉ cũ/mới, geocoding, reverse geocoding và các tín hiệu vị trí phục vụ logistics.

Thiết kế phải xử lý được đặc thù dữ liệu Việt Nam: mô hình hành chính 2 cấp/3 cấp, thay đổi địa giới theo thời gian, tên cũ/tên viết tắt, đường/hẻm/thôn/ấp/khu phố, POI/tòa nhà, dữ liệu OpenStreetMap và quan hệ không thuần cây giữa place với đơn vị hành chính.

### 1.2 Trong phạm vi

- Canonical master data cho đơn vị hành chính, place và geographic data.
- Temporal history cho thay đổi hành chính.
- Alias và external identity/provenance.
- OSM ingestion mapping vào canonical model.
- PostGIS geometry và spatial relationship.
- Elasticsearch read model cho autocomplete/search/ranking.
- Điểm giao/tiếp cận đã tổng hợp hoặc xác minh cho logistics.

### 1.3 Ngoài phạm vi

- Không materialize trước toàn bộ tổ hợp `số nhà + đường` của Việt Nam thành bảng `addresses`.
- Không lưu raw GPS event của shipper trong database canonical này.
- Không coi Elasticsearch `_score`, query cache, recent search hoặc user-specific ranking là master data.
- Không mô tả chi tiết API contract, deployment topology hoặc thuật toán NLP/parser; các phần này thuộc tài liệu thiết kế tương ứng.

------------------------------------------------------------------------

## 2. Architectural Drivers và quyết định nền tảng

| ID        | Quyết định                                                                                            | Lý do                                                                                    |
|:----------|:------------------------------------------------------------------------------------------------------|:-----------------------------------------------------------------------------------------|
| ADR-DB-01 | PostgreSQL + PostGIS là canonical source of truth                                                     | Cần transaction, constraint, temporal data và spatial query đáng tin cậy                 |
| ADR-DB-02 | Elasticsearch là derived read model                                                                   | Tối ưu prefix/fuzzy/full-text/geo ranking và có thể rebuild                              |
| ADR-DB-03 | Administrative Unit tách khỏi Place                                                                   | Đường, POI, building không phải đơn vị hành chính                                        |
| ADR-DB-04 | MVP place-centric, không có Address Domain canonical                                                 | Canonical identity chỉ thuộc Administrative Unit, Place/Street, Geometry và Delivery Point; `house_number` và chuỗi địa chỉ do parser/resolver xử lý |
| ADR-DB-05 | OSM là external source, không phải canonical schema                                                   | Tránh khóa domain model vào `node/way/relation` và OSM ID                                |
| ADR-DB-06 | Boundary, place geometry và delivery point tách riêng                                                 | Khác semantic, nguồn, vòng đời và query pattern                                          |
| ADR-DB-07 | Lịch sử hành chính dùng change + members                                                              | Hỗ trợ rename, merge, split và boundary adjustment                                       |
| ADR-DB-08 | Internal ID ổn định, external identity lưu riêng                                                      | Cho phép đổi/ghép nguồn mà không phá quan hệ nội bộ                                      |
| ADR-DB-09 | Temporal validity phải chống overlap ở database                                                       | `UNIQUE(unit_code, valid_from)` không ngăn được khoảng hiệu lực chồng lấn                |
| ADR-DB-10 | `external_references` dùng FK tường minh + exactly-one constraint                                     | Tránh polymorphic `(entity_type, entity_id)` không enforce được referential integrity    |
| ADR-DB-11 | Một canonical Place có thể sinh nhiều Elasticsearch search-context document                           | Hỗ trợ Place/đường đi qua nhiều administrative context mà không phá canonical identity   |
| ADR-DB-12 | Incremental indexing dùng domain/search events; full reindex dùng versioned index + atomic alias swap | Tách near-real-time sync khỏi rebuild/rollback và tránh downtime                         |
| ADR-DB-13 | MVP dùng Elasticsearch duy nhất; không dùng Redis, broker, CDC hoặc LISTEN/NOTIFY                     | Giữ baseline vận hành nhỏ; chỉ bổ sung sau benchmark và SLO thực tế                       |

### 2.1 Nguyên tắc thiết kế

1.  Dữ liệu canonical phải có ownership rõ ràng và truy vết được nguồn.
2.  Constraint quan trọng phải được enforce ở database khi khả thi, không chỉ bằng application code.
3.  Quan hệ và temporal validity phải biểu diễn đúng thực tế, không ép dữ liệu Việt Nam vào hierarchy cố định.
4.  Search-specific data phải có khả năng tái tạo từ canonical data.
5.  Geometry canonical dùng SRID 4326; mọi phép đo khoảng cách phải dùng đơn vị rõ ràng.
6.  Schema ưu tiên correctness, auditability, maintainability và operability hơn tối ưu sớm.

## 3. Context và Data Ownership

### 3.1 Data Context

``` mermaid
flowchart LR
    GOV["Nguồn hành chính"]
    OSM["OpenStreetMap"]
    INT["SuperShip Internal / Delivery-derived"]

    INGEST["Ingestion / Validation / Mapping"]
    PG[("PostgreSQL + PostGIS\nCanonical Data")]
    BUILD["Index Builder"]
    ES[("Elasticsearch\nDerived Read Model")]
    API["Address Autocomplete / Resolve API"]

    GOV --> INGEST
    OSM --> INGEST
    INT --> INGEST
    INGEST --> PG
    PG --> BUILD
    BUILD --> ES
    ES --> API
```

### 3.2 Ownership

| Thành phần            | Vai trò                               |          Được phép là source of truth? |
|:----------------------|:--------------------------------------|---------------------------------------:|
| PostgreSQL            | Relational canonical data             |                                     Có |
| PostGIS               | Canonical spatial data                |                                     Có |
| Elasticsearch         | Search/read model                     |                                  Không |
| OpenStreetMap         | External data source                  |                                  Không |
| Delivery-derived data | Tín hiệu nội bộ sau tổng hợp/xác minh | Chỉ khi đã promote vào canonical model |

**Serving rule:** autocomplete đọc Elasticsearch; không join PostgreSQL trong từng keystroke. PostgreSQL commit và Elasticsearch indexing là hai consistency boundary khác nhau, do đó chấp nhận eventual consistency và phải hỗ trợ reindex.

------------------------------------------------------------------------

## 4. Mô hình dữ liệu khái niệm

### 4.1 Bounded Data Domains

``` text
Address Autocomplete Data Model
│
├── Administrative
│   ├── administrative_units
│   └── administrative_unit_aliases
│
├── Administrative History
│   ├── administrative_changes
│   └── administrative_change_members
│
├── Place
│   ├── places
│   ├── place_aliases
│   ├── place_admin_relations
│   └── place_relations
│
├── Geographic
│   ├── geo_boundaries
│   ├── place_geometries
│   └── delivery_points
│
├── Data Governance
│   ├── data_sources
│   └── external_references
│
└── Search Read Model
    └── Elasticsearch documents
```

### 4.2 Khái niệm cốt lõi

``` text
Administrative Unit ≠ Place ≠ Search Document
```

- **Administrative Unit:** tỉnh/thành, quận/huyện lịch sử, phường/xã và các đơn vị hành chính có hiệu lực theo thời gian.
- **Place:** đường, hẻm, thôn/ấp, khu dân cư, building, chung cư, POI, landmark…
- **Geographic:** hình học và điểm vị trí gắn với administrative unit/place.
- **Search Document:** bản flatten phục vụ retrieval/ranking, có thể rebuild và không phải canonical entity.

`house_number` được coi là **query/address component** do parser/resolver xử lý. Nếu sau này SuperShip xây dựng Verified Address từ lịch sử giao hàng, đó nên là một bounded context riêng thay vì ép vào core model hiện tại.

------------------------------------------------------------------------

## 5. Logical ERD

``` mermaid
erDiagram
    DATA_SOURCES ||--o{ ADMINISTRATIVE_UNITS : provides
    DATA_SOURCES ||--o{ PLACES : provides
    DATA_SOURCES ||--o{ GEO_BOUNDARIES : provides
    DATA_SOURCES ||--o{ PLACE_GEOMETRIES : provides
    DATA_SOURCES ||--o{ DELIVERY_POINTS : provides
    DATA_SOURCES ||--o{ EXTERNAL_REFERENCES : identifies

    ADMINISTRATIVE_UNITS o|--o{ ADMINISTRATIVE_UNITS : parent
    ADMINISTRATIVE_UNITS ||--o{ ADMINISTRATIVE_UNIT_ALIASES : has
    ADMINISTRATIVE_UNITS ||--o{ ADMINISTRATIVE_CHANGE_MEMBERS : participates
    ADMINISTRATIVE_UNITS ||--o{ PLACE_ADMIN_RELATIONS : relates
    ADMINISTRATIVE_UNITS ||--o{ GEO_BOUNDARIES : has

    ADMINISTRATIVE_CHANGES ||--|{ ADMINISTRATIVE_CHANGE_MEMBERS : contains

    PLACES o|--o{ PLACES : parent
    PLACES ||--o{ PLACE_ALIASES : has
    PLACES ||--o{ PLACE_ADMIN_RELATIONS : relates
    PLACES ||--o{ PLACE_RELATIONS : source
    PLACES ||--o{ PLACE_RELATIONS : target
    PLACES ||--o{ PLACE_GEOMETRIES : has
    PLACES ||--o{ DELIVERY_POINTS : has
```

**Quy ước ERD:** sơ đồ chỉ thể hiện entity và relationship chính. Data type, nullability, unique/check constraint và index được đặc tả tại Data Dictionary để tránh trộn nhiều mức abstraction trong một hình.

------------------------------------------------------------------------

## 6. Đặc tả Domain

### 6.1 Administrative Domain

#### 6.1.1 `administrative_units`

Lưu **đơn vị hành chính**, không lưu đường, hẻm, tòa nhà hay POI.

Ví dụ:

``` text
Thành phố Hồ Chí Minh
├── Quận 1                    (historical / nếu thuộc mô hình 3 cấp)
│   └── Phường Bến Nghé
└── Phường ...                (có thể nối trực tiếp trong mô hình 2 cấp)
```

#### Data Dictionary

| Field             | Type         | Null | Ý nghĩa                                      |
|:------------------|:-------------|-----:|:---------------------------------------------|
| `id`              | bigint PK    |   No | ID nội bộ, ổn định                           |
| `unit_code`       | varchar(50)  |   No | Mã đơn vị hành chính theo nguồn chuẩn        |
| `unit_name`       | varchar(255) |   No | Tên chính thức                               |
| `normalized_name` | varchar(255) |   No | Tên đã normalize phục vụ xử lý               |
| `unit_type`       | varchar(30)  |   No | PROVINCE, CITY, DISTRICT, WARD, COMMUNE…     |
| `admin_level`     | smallint     |   No | Cấp hành chính                               |
| `parent_id`       | bigint FK    |  Yes | Đơn vị cha trong hierarchy của phiên bản này |
| `status`          | varchar(20)  |   No | ACTIVE, INACTIVE                             |
| `valid_from`      | date         |  Yes | Ngày bắt đầu hiệu lực                        |
| `valid_to`        | date         |  Yes | Ngày hết hiệu lực; NULL = còn hiệu lực       |
| `source_id`       | bigint FK    |  Yes | Nguồn dữ liệu                                |
| `created_at`      | timestamptz  |   No | Thời điểm tạo                                |
| `updated_at`      | timestamptz  |   No | Thời điểm cập nhật                           |

#### Constraint chính

``` text
UNIQUE(unit_code, valid_from)

parent_id <> id

valid_to IS NULL OR valid_to >= valid_from
```

> Không nên mã hóa cứng rằng level 2 luôn phải tồn tại. Mô hình 2 cấp và 3 cấp được biểu diễn bằng chính quan hệ `parent_id`.

#### 6.1.2 `administrative_unit_aliases`

Lưu tên cũ, viết tắt, tên thường gọi và biến thể tìm kiếm của đơn vị hành chính.

| Field              | Type         | Null | Ý nghĩa              |
|:-------------------|:-------------|-----:|:---------------------|
| `id`               | bigint PK    |   No | ID                   |
| `admin_unit_id`    | bigint FK    |   No | Đơn vị hành chính    |
| `alias`            | varchar(255) |   No | Alias hiển thị       |
| `normalized_alias` | varchar(255) |   No | Alias đã normalize   |
| `alias_type`       | varchar(30)  |   No | Loại alias           |
| `priority`         | smallint     |   No | Độ ưu tiên tìm kiếm  |
| `valid_from`       | date         |  Yes | Bắt đầu hiệu lực     |
| `valid_to`         | date         |  Yes | Kết thúc hiệu lực    |
| `source_id`        | bigint FK    |  Yes | Nguồn                |
| `verified`         | boolean      |   No | Đã xác minh hay chưa |
| `created_at`       | timestamptz  |   No | Ngày tạo             |

#### `alias_type`

``` text
OFFICIAL
OLD_NAME
SHORT_NAME
ABBREVIATION
COMMON_NAME
ALTERNATIVE_SPELLING
SEARCH_SYNONYM
```

Ví dụ:

``` text
Thành phố Hồ Chí Minh
├── TP.HCM       → ABBREVIATION
├── TPHCM        → ABBREVIATION
├── HCM          → ABBREVIATION
└── Sài Gòn      → COMMON_NAME / historical context
```

### 6.2 Administrative Change Domain

#### 6.2.1 Vì sao không dùng chỉ `old_code → new_code`?

Thực tế cần biểu diễn:

``` text
Đổi tên:       A → B

Sáp nhập:      A ─┐
               B ─┼→ X
               C ─┘

Chia tách:        ┌→ B
               A ─┼→ C
                  └→ D
```

Vì vậy lịch sử được mô hình hóa thành **change event + members**.

#### 6.2.2 `administrative_changes`

| Field                | Type         | Null | Ý nghĩa                  |
|:---------------------|:-------------|-----:|:-------------------------|
| `id`                 | bigint PK    |   No | ID sự kiện               |
| `change_type`        | varchar(30)  |   No | Loại thay đổi            |
| `effective_date`     | date         |   No | Ngày hiệu lực            |
| `resolution_no`      | varchar(100) |  Yes | Số nghị quyết/quyết định |
| `description`        | text         |  Yes | Mô tả                    |
| `source_id`          | bigint FK    |  Yes | Nguồn                    |
| `created_by_user_id` | bigint       |  Yes | Người nhập dữ liệu       |
| `created_at`         | timestamptz  |   No | Ngày tạo                 |

#### `change_type`

``` text
RENAME
MERGE
SPLIT
BOUNDARY_ADJUSTMENT
LEVEL_CHANGE
CODE_CHANGE
CREATE
DISSOLVE
OTHER
```

#### 6.2.3 `administrative_change_members`

| Field           | Type        | Null | Ý nghĩa            |
|:----------------|:------------|-----:|:-------------------|
| `id`            | bigint PK   |   No | ID                 |
| `change_id`     | bigint FK   |   No | Sự kiện thay đổi   |
| `admin_unit_id` | bigint FK   |   No | Đơn vị tham gia    |
| `role`          | varchar(10) |   No | SOURCE hoặc TARGET |
| `note`          | text        |  Yes | Ghi chú            |

Ví dụ sáp nhập:

``` text
Change #1001 — MERGE

SOURCE → Phường A
SOURCE → Phường B
SOURCE → Phường C
TARGET → Phường X
```

Thiết kế này biểu diễn được N→1, 1→N và N→N.

### 6.3 Place Domain

#### 6.3.1 `places`

Lưu các đối tượng địa chỉ **không phải đơn vị hành chính**.

Ví dụ:

``` text
STREET
ALLEY
HAMLET
VILLAGE
RESIDENTIAL_AREA
BUILDING
APARTMENT_COMPLEX
INDUSTRIAL_ZONE
MARKET
SCHOOL
HOSPITAL
LANDMARK
POI
```

#### Data Dictionary

| Field             | Type                 | Null | Ý nghĩa                                                                                |
|:------------------|:---------------------|-----:|:---------------------------------------------------------------------------------------|
| `id`              | bigint PK            |   No | ID                                                                                     |
| `place_code`      | varchar(100)         |  Yes | Mã nội bộ/nguồn                                                                        |
| `place_name`      | varchar(255)         |   No | Tên                                                                                    |
| `normalized_name` | varchar(255)         |   No | Tên normalize                                                                          |
| `place_type`      | varchar(40)          |   No | Loại địa điểm                                                                          |
| `parent_place_id` | bigint FK            |  Yes | Place cha nếu có                                                                       |
| `location`        | geometry(Point,4326) |  Yes | Điểm đại diện/centroid nhẹ để truy vấn nhanh; geometry đầy đủ nằm ở `place_geometries` |
| `status`          | varchar(20)          |   No | ACTIVE/INACTIVE                                                                        |
| `valid_from`      | date                 |  Yes | Hiệu lực từ                                                                            |
| `valid_to`        | date                 |  Yes | Hiệu lực đến                                                                           |
| `source_id`       | bigint FK            |  Yes | Nguồn                                                                                  |
| `created_at`      | timestamptz          |   No | Ngày tạo                                                                               |
| `updated_at`      | timestamptz          |   No | Ngày cập nhật                                                                          |

Ví dụ hierarchy:

``` text
Chung cư ABC
├── Tòa A
│   ├── Sảnh A1
│   └── Sảnh A2
└── Tòa B
```

#### 6.3.2 `place_aliases`

| Field              | Type         | Null | Ý nghĩa         |
|:-------------------|:-------------|-----:|:----------------|
| `id`               | bigint PK    |   No | ID              |
| `place_id`         | bigint FK    |   No | Place           |
| `alias`            | varchar(255) |   No | Tên khác        |
| `normalized_alias` | varchar(255) |   No | Alias normalize |
| `alias_type`       | varchar(30)  |   No | Loại alias      |
| `priority`         | smallint     |   No | Ưu tiên         |
| `source_id`        | bigint FK    |  Yes | Nguồn           |
| `verified`         | boolean      |   No | Đã xác minh     |
| `created_at`       | timestamptz  |   No | Ngày tạo        |

### 6.4 Place - Administrative Relation Domain

#### 6.4.1 `place_admin_relations`

Một place không nhất thiết chỉ thuộc một phường.

Ví dụ:

``` text
Đường Nguyễn Trãi
├── INTERSECTS → Phường A
├── INTERSECTS → Phường B
└── INTERSECTS → Phường C
```

#### Data Dictionary

| Field           | Type        | Null | Ý nghĩa              |
|:----------------|:------------|-----:|:---------------------|
| `id`            | bigint PK   |   No | ID                   |
| `place_id`      | bigint FK   |   No | Place                |
| `admin_unit_id` | bigint FK   |   No | Đơn vị hành chính    |
| `relation_type` | varchar(30) |   No | Quan hệ              |
| `is_primary`    | boolean     |   No | Context chính nếu có |
| `valid_from`    | date        |  Yes | Hiệu lực từ          |
| `valid_to`      | date        |  Yes | Hiệu lực đến         |
| `source_id`     | bigint FK   |  Yes | Nguồn                |

#### `relation_type`

``` text
WITHIN
INTERSECTS
SERVES
NEAR
```

Thông thường dữ liệu canonical ưu tiên `WITHIN` và `INTERSECTS`; `NEAR` nên chỉ dùng khi có use case rõ ràng.

### 6.5 Place Relation Domain

#### 6.5.1 `place_relations`

Dùng khi hai địa điểm có quan hệ không phù hợp với `parent_place_id`.

| Field             | Type        | Null | Ý nghĩa      |
|:------------------|:------------|-----:|:-------------|
| `id`              | bigint PK   |   No | ID           |
| `source_place_id` | bigint FK   |   No | Place nguồn  |
| `target_place_id` | bigint FK   |   No | Place đích   |
| `relation_type`   | varchar(30) |   No | Loại quan hệ |
| `source_id`       | bigint FK   |  Yes | Nguồn        |
| `created_at`      | timestamptz |   No | Ngày tạo     |

Ví dụ:

``` text
ENTRANCE_OF
PART_OF
CONNECTED_TO
NEAR
```

Ví dụ logistics:

``` text
Cổng số 2 ──ENTRANCE_OF──> Chung cư ABC
```

#### Quyết định: không materialize toàn bộ Address Domain

Trong scope hiện tại, hệ thống **không duy trì `addresses` và `address_components` như canonical tables bắt buộc**.

Một chuỗi như:

``` text
12 Nguyễn Trãi, Bến Thành, TP.HCM
```

được xử lý theo hướng:

``` text
house_number = 12              ← query/parser component
street       = Nguyễn Trãi     ← Place
ward/city                     ← Administrative Unit
tọa độ/geometry               ← Geographic Domain
```

Không tạo trước một row canonical cho mọi tổ hợp số nhà + đường trên toàn Việt Nam.

Địa chỉ cụ thể đã được xác minh từ vận hành/giao hàng có thể được bổ sung sau dưới một bounded context riêng như **Verified Address / Delivery Address Intelligence**. Việc này không phải dependency của autocomplete ở giai đoạn hiện tại.

### 6.6 Geographic Domain

Geographic Domain là canonical spatial layer của hệ thống. **PostGIS chịu trách nhiệm dữ liệu địa lý chuẩn**; Elasticsearch chỉ nhận bản sao các geo field cần thiết cho autocomplete, filter và ranking.

#### 6.6.1 `geo_boundaries`

Lưu **địa giới hành chính** theo phiên bản. Không dùng bảng này để lưu hình học của đường, tòa nhà hay POI.

| Field           | Type                        | Null | Ý nghĩa           |
|:----------------|:----------------------------|-----:|:------------------|
| `id`            | bigint PK                   |   No | ID                |
| `admin_unit_id` | bigint FK                   |   No | Đơn vị hành chính |
| `boundary`      | geometry(MultiPolygon,4326) |   No | Biên địa giới     |
| `centroid`      | geometry(Point,4326)        |  Yes | Tâm/điểm đại diện |
| `valid_from`    | date                        |  Yes | Hiệu lực từ       |
| `valid_to`      | date                        |  Yes | Hiệu lực đến      |
| `source_id`     | bigint FK                   |  Yes | Nguồn             |
| `created_at`    | timestamptz                 |   No | Ngày tạo          |
| `updated_at`    | timestamptz                 |   No | Ngày cập nhật     |

Ứng dụng:

``` text
GPS → thuộc tỉnh/phường nào?
Boundary cũ/mới → hiệu lực thời điểm nào?
Địa chỉ user chọn → có khớp GPS không?
```

#### 6.6.2 `place_geometries`

Lưu geometry đầy đủ của đối tượng không phải đơn vị hành chính.

| Field           | Type                    | Null | Ý nghĩa                                     |
|:----------------|:------------------------|-----:|:--------------------------------------------|
| `id`            | bigint PK               |   No | ID                                          |
| `place_id`      | bigint FK               |   No | Place                                       |
| `geometry_type` | varchar(20)             |   No | POINT / LINESTRING / POLYGON / MULTIPOLYGON |
| `geometry`      | geometry(Geometry,4326) |   No | Geometry PostGIS                            |
| `is_primary`    | boolean                 |   No | Geometry chính của place                    |
| `valid_from`    | date                    |  Yes | Hiệu lực từ                                 |
| `valid_to`      | date                    |  Yes | Hiệu lực đến                                |
| `source_id`     | bigint FK               |  Yes | Nguồn                                       |
| `created_at`    | timestamptz             |   No | Ngày tạo                                    |
| `updated_at`    | timestamptz             |   No | Ngày cập nhật                               |

Ví dụ ánh xạ dữ liệu:

``` text
STREET / ROAD        → LINESTRING
BUILDING             → POLYGON
INDUSTRIAL_ZONE      → POLYGON / MULTIPOLYGON
POI                  → POINT
LANDMARK             → POINT / POLYGON
```

Một place có thể có nhiều geometry theo nguồn hoặc phiên bản; chỉ một geometry đang hiệu lực nên được đánh dấu `is_primary=true`.

#### 6.6.3 `delivery_points`

Lưu **điểm tiếp cận thực tế cho logistics**, tách khỏi centroid hoặc geometry của địa điểm.

| Field                   | Type                 | Null | Ý nghĩa                                                                     |
|:------------------------|:---------------------|-----:|:----------------------------------------------------------------------------|
| `id`                    | bigint PK            |   No | ID                                                                          |
| `place_id`              | bigint FK            |  Yes | Place liên quan                                                             |
| `point_type`            | varchar(30)          |   No | ENTRANCE / DELIVERY / PICKUP / LOADING_GATE / WAREHOUSE_GATE / ACCESS_POINT |
| `location`              | geometry(Point,4326) |   No | Điểm giao/tiếp cận                                                          |
| `confidence`            | numeric(5,4)         |  Yes | Độ tin cậy 0..1                                                             |
| `verification_status`   | varchar(20)          |   No | UNVERIFIED / VERIFIED / REJECTED                                            |
| `source_id`             | bigint FK            |  Yes | Nguồn                                                                       |
| `successful_deliveries` | bigint               |   No | Số lượt giao thành công đã tổng hợp                                         |
| `last_verified_at`      | timestamptz          |  Yes | Lần xác minh gần nhất                                                       |
| `created_at`            | timestamptz          |   No | Ngày tạo                                                                    |
| `updated_at`            | timestamptz          |   No | Ngày cập nhật                                                               |

Ví dụ:

``` text
Chung cư ABC
├── building polygon
├── centroid
├── ENTRANCE
├── DELIVERY
└── LOADING_GATE
```

`delivery_points` không thay thế raw GPS event của shipper. Dữ liệu GPS sự kiện nên nằm ở hệ thống vận hành/analytics riêng; bảng này chỉ lưu **điểm đã được tổng hợp/xác minh** phục vụ định vị và giao nhận.

#### 6.6.4 Quy tắc Geographic

``` text
Administrative boundary → geo_boundaries
Place geometry          → place_geometries
Điểm giao/tiếp cận      → delivery_points
Điểm đại diện nhẹ       → places.location
```

Các spatial index chính:

``` sql
CREATE INDEX idx_geo_boundaries_boundary_gist
ON geo_boundaries USING GIST(boundary);

CREATE INDEX idx_place_geometries_geometry_gist
ON place_geometries USING GIST(geometry);

CREATE INDEX idx_delivery_points_location_gist
ON delivery_points USING GIST(location);
```

### 6.7 Data Source / Governance Domain

#### 6.7.1 `data_sources`

Mọi dữ liệu quan trọng nên biết đến từ đâu.

| Field          | Type           | Null | Ý nghĩa                                     |
|:---------------|:---------------|-----:|:--------------------------------------------|
| `id`           | bigint PK      |   No | ID                                          |
| `source_code`  | varchar(50) UK |   No | Mã nguồn, ví dụ OSM                         |
| `source_name`  | varchar(255)   |   No | Tên nguồn                                   |
| `source_type`  | varchar(30)    |   No | OPEN_DATA / GOVERNMENT / INTERNAL / PARTNER |
| `reference`    | text           |  Yes | Tham chiếu dataset                          |
| `version`      | varchar(100)   |  Yes | Phiên bản/snapshot                          |
| `published_at` | timestamptz    |  Yes | Ngày công bố                                |
| `imported_at`  | timestamptz    |  Yes | Ngày import                                 |
| `metadata`     | jsonb          |  Yes | Metadata bổ sung                            |

Ví dụ:

``` text
OSM
GOVERNMENT_ADMIN
SUPERSHIP_INTERNAL
DELIVERY_DERIVED
```

#### 6.7.2 `external_references`

Lưu identity của entity trong hệ thống nguồn để phục vụ incremental sync, reconciliation và audit. Không dùng OSM ID hoặc external ID làm primary key của canonical entity.

**Khuyến nghị production:** không dùng cặp polymorphic `(entity_type, entity_id)` vì PostgreSQL không thể tạo Foreign Key thực sự tới nhiều bảng. Thay vào đó, dùng các FK nullable tường minh và enforce chính xác một target bằng `num_nonnulls()`.

| Cột                 | Kiểu dữ liệu   | Null | Mô tả                                                   |
|:--------------------|:---------------|:-----|:--------------------------------------------------------|
| `id`                | `bigint`       | No   | Primary key                                             |
| `source_id`         | `bigint`       | No   | FK → `data_sources.id`                                  |
| `admin_unit_id`     | `bigint`       | Yes  | FK → `administrative_units.id`                          |
| `place_id`          | `bigint`       | Yes  | FK → `places.id`                                        |
| `geo_boundary_id`   | `bigint`       | Yes  | FK → `geo_boundaries.id`                                |
| `delivery_point_id` | `bigint`       | Yes  | FK → `delivery_points.id`                               |
| `external_type`     | `varchar(30)`  | No   | Loại object phía nguồn, ví dụ `NODE`, `WAY`, `RELATION` |
| `external_id`       | `varchar(100)` | No   | ID của object phía nguồn                                |
| `external_version`  | `varchar(100)` | Yes  | Version/revision của object phía nguồn                  |
| `last_synced_at`    | `timestamptz`  | Yes  | Lần đồng bộ gần nhất                                    |
| `metadata`          | `jsonb`        | Yes  | Metadata bổ sung từ nguồn                               |
| `created_at`        | `timestamptz`  | No   | Thời điểm tạo                                           |
| `updated_at`        | `timestamptz`  | No   | Thời điểm cập nhật                                      |

``` sql
CREATE TABLE external_references (
    id                  BIGSERIAL PRIMARY KEY,
    source_id           BIGINT NOT NULL REFERENCES data_sources(id),
    admin_unit_id       BIGINT REFERENCES administrative_units(id),
    place_id            BIGINT REFERENCES places(id),
    geo_boundary_id     BIGINT REFERENCES geo_boundaries(id),
    delivery_point_id   BIGINT REFERENCES delivery_points(id),
    external_type       VARCHAR(30) NOT NULL,
    external_id         VARCHAR(100) NOT NULL,
    external_version    VARCHAR(100),
    last_synced_at      TIMESTAMPTZ,
    metadata            JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT ck_external_reference_exactly_one_entity
        CHECK (
            num_nonnulls(
                admin_unit_id,
                place_id,
                geo_boundary_id,
                delivery_point_id
            ) = 1
        ),

    CONSTRAINT uq_external_reference_source_object
        UNIQUE (source_id, external_type, external_id)
);
```

**Delete policy:** không mặc định dùng `ON DELETE CASCADE`. Administrative Unit và Place nên ưu tiên trạng thái `INACTIVE`, `MERGED`, `SUPERSEDED` hoặc temporal validity thay vì hard delete. Cách này giữ provenance để audit/reconciliation.

#### 6.7.3 Luồng OpenStreetMap

``` mermaid
flowchart LR
    OSM["OpenStreetMap"]
    INGEST["OSM Import / ETL"]
    MAP["Normalize + Entity Mapping"]
    PG[("PostgreSQL + PostGIS")]
    IDX["Index Builder"]
    ES[("Elasticsearch")]

    OSM --> INGEST
    INGEST --> MAP
    MAP --> PG
    PG --> IDX
    IDX --> ES
```

Nguyên tắc:

- Không copy nguyên schema OSM thành domain model của SuperShip.
- `node/way/relation` là khái niệm nguồn; canonical model vẫn là Admin Unit / Place / Address / Geographic.
- Giữ OSM identity trong `external_references` để incremental sync, audit và reconciliation.
- Khi OSM thay đổi, ETL phải map lại vào canonical entity thay vì thay ID nội bộ.
- Dữ liệu nội bộ có thể có độ ưu tiên cao hơn OSM cho delivery point đã được xác minh.

Điều này giúp trả lời:

``` text
Tên phường này lấy từ nguồn nào?
Boundary phiên bản nào?
Place này map tới OSM node/way/relation nào?
Alias do hệ thống hay nguồn ngoài tạo?
Delivery point do OSM hay dữ liệu giao hàng xác minh?
```

------------------------------------------------------------------------

------------------------------------------------------------------------

## 7. Temporal và Historical Data

### 7.1 Nguyên tắc temporal

Dữ liệu hành chính phải phân biệt:

- **Identity:** entity nào đang được mô tả.
- **Business validity:** entity/version có hiệu lực trong khoảng thời gian nào.
- **System timestamps:** record được tạo/cập nhật trong hệ thống khi nào.

`valid_from` / `valid_to` biểu diễn hiệu lực nghiệp vụ. `created_at` / `updated_at` chỉ biểu diễn thời điểm record được ghi vào hệ thống.

### 7.2 Không dùng `UNIQUE(unit_code, valid_from)` để chống overlap

Ràng buộc sau không đủ mạnh:

``` sql
UNIQUE (unit_code, valid_from)
```

Lý do:

1.  `UNIQUE` không ngăn hai khoảng thời gian khác `valid_from` nhưng vẫn chồng lấn.
2.  Nếu `valid_from` cho phép `NULL`, nhiều row có thể cùng tồn tại theo semantics mặc định của PostgreSQL.
3.  Historical resolver cần invariant mạnh hơn: tại một thời điểm, cùng một `unit_code` không được có hai version cùng hiệu lực.

### 7.3 Quy ước khoảng hiệu lực

Ưu tiên khoảng half-open:

``` text
[valid_from, valid_to)
```

Ví dụ:

``` text
Version A: [2020-01-01, 2025-01-01)
Version B: [2025-01-01, infinity)
```

Hai version nối tiếp nhau nhưng không overlap.

Không tự gán ngày giả như `1970-01-01` khi ngày bắt đầu thực tế chưa biết. Nếu business bắt buộc `valid_from NOT NULL`, giá trị phải có provenance; nếu không biết, cần biểu diễn trạng thái unknown/partial-history rõ ràng.

### 7.4 PostgreSQL \<= 17: Exclusion Constraint

``` sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE administrative_units
ADD CONSTRAINT ex_admin_unit_no_overlap
EXCLUDE USING gist (
    unit_code WITH =,
    daterange(valid_from, valid_to, '[)') WITH &&
);
```

Range bound `NULL` có thể biểu diễn unbounded range; không cần dùng ngày giả để đại diện infinity.

### 7.5 PostgreSQL 18+: Temporal constraint

Nếu runtime chuẩn hóa trên PostgreSQL 18+, có thể cân nhắc temporal key/`WITHOUT OVERLAPS` để biểu diễn trực tiếp invariant không chồng lấn. Quyết định cuối cùng phải theo version PostgreSQL thực tế của production.

### 7.6 Change model

``` text
administrative_changes
        │
        └── administrative_change_members
              ├── SOURCE
              └── TARGET
```

| Trường hợp          | SOURCE | TARGET |
|:--------------------|-------:|-------:|
| Rename              |      1 |      1 |
| Merge               |      N |      1 |
| Split               |      1 |      N |
| Boundary adjustment |      N |      N |

Không dùng duy nhất `old_code → new_code`, vì mô hình đó không biểu diễn đúng merge, split hoặc N→N.

### 7.7 Hành chính 2 cấp và 3 cấp

``` text
3 cấp:
Province / City
└── District
    └── Ward / Commune

2 cấp:
Province / City
└── Ward / Commune
```

Schema không hard-code việc ward bắt buộc phải có district. `parent_id` và temporal validity quyết định hierarchy hợp lệ tại từng thời điểm.

## 8. Geographic Data Model

### 8.1 Phân vai geometry

| Dữ liệu                    | Bảng               | Kiểu geometry điển hình    |
|:---------------------------|:-------------------|:---------------------------|
| Địa giới hành chính        | `geo_boundaries`   | `MultiPolygon`             |
| Đường                      | `place_geometries` | `LineString`               |
| Building / KCN             | `place_geometries` | `Polygon` / `MultiPolygon` |
| POI / Landmark             | `place_geometries` | `Point` / `Polygon`        |
| Entrance / Delivery / Gate | `delivery_points`  | `Point`                    |
| Điểm đại diện nhanh        | `places.location`  | `Point`                    |

### 8.2 OSM mapping

``` mermaid
flowchart LR
    OSM["OSM node / way / relation"]
    ETL["Import + Normalize"]
    MAP["Entity Matching / Mapping"]
    CANON["Canonical Entity"]
    REF["external_references"]

    OSM --> ETL
    ETL --> MAP
    MAP --> CANON
    MAP --> REF
```

`node`, `way`, `relation` là khái niệm của OSM. Canonical model vẫn là Administrative Unit / Place / Geographic. OSM identity chỉ được giữ trong `external_references`.

### 8.3 `geometry` và `geography`

Canonical geometry tiếp tục dùng SRID 4326:

``` sql
geometry(Point, 4326)
geometry(LineString, 4326)
geometry(MultiPolygon, 4326)
```

Với `geometry` trên EPSG:4326, không giả định khoảng cách trả về theo mét.

**Chiến lược A — cast sang `geography`:**

``` sql
ST_DWithin(
    location::geography,
    ST_SetSRID(ST_MakePoint(:lng, :lat), 4326)::geography,
    500
)
```

Trong ví dụ trên, `500` là 500 mét.

**Chiến lược B — transform sang projected CRS phù hợp:**

``` sql
ST_DWithin(
    ST_Transform(location, :projected_srid),
    ST_Transform(
        ST_SetSRID(ST_MakePoint(:lng, :lat), 4326),
        :projected_srid
    ),
    500
)
```

Không đổi toàn bộ canonical spatial model sang `geography` chỉ vì có radius query. `geometry` linh hoạt hơn cho spatial operation; `geography` phù hợp khi cần khoảng cách theo mét trên bề mặt Trái Đất.

### 8.4 Spatial rules

- Canonical geometry dùng SRID 4326.
- Geometry phải hợp lệ trước khi promote thành canonical.
- Point-in-polygon, intersects/contains và reverse-geocode nền tảng chạy trên PostGIS.
- Geo proximity trong autocomplete là ranking signal, không mặc nhiên là hard filter.
- GPS không mặc nhiên là bằng chứng tuyệt đối cho administrative address nếu boundary/source chưa đủ chất lượng.
- Spatial index phải khớp với expression được query. Nếu production thường xuyên query `location::geography`, cần benchmark và cân nhắc expression index.

## 9. Search Read Model

### 9.1 Mục tiêu

Elasticsearch document phải **denormalized, search-oriented và rebuildable**. Không thiết kế index như bản sao 1:1 của relational schema.

### 9.2 Canonical entity và search-context document

Một canonical Place có thể thuộc hoặc giao cắt nhiều administrative context. Không tách canonical Place thành nhiều Place chỉ để phục vụ search.

``` text
1 Canonical Place
        │
        ├── Search Context A
        ├── Search Context B
        └── Search Context C
```

Ví dụ:

``` text
place:100:admin:A → Nguyễn Trãi, Phường A
place:100:admin:B → Nguyễn Trãi, Phường B
place:100:admin:C → Nguyễn Trãi, Phường C
```

Mỗi search-context document có thể có:

- `canonical_place_id`
- `search_context_id`
- `display_name`
- `admin_path`
- `aliases`
- `historical_names`
- `location`
- `geometry_summary`
- `popularity`
- `source_quality`
- `logistics_signals`

Chiến lược này cho phép display text, geo context và ranking bám đúng context đang match mà không phá canonical identity.

### 9.3 Khi nào dùng Elasticsearch `nested`

Nếu giữ một document duy nhất với nhiều object admin context, `nested` có thể dùng để tránh cross-object matching sai. Tuy nhiên `nested` chỉ giải quyết correlation khi query; nó không tự giải quyết display context hoặc geo context.

Với autocomplete, mặc định ưu tiên **search-context documents**. Chỉ dùng `nested` khi có lý do rõ ràng và đã benchmark query/indexing cost.

### 9.4 Document gợi ý

``` json
{
  "document_id": "place:100:admin:200",
  "canonical_place_id": 100,
  "entity_type": "STREET",
  "display_name": "Nguyễn Trãi, Phường A",
  "normalized_name": "nguyen trai",
  "aliases": ["ng trai", "duong nguyen trai"],
  "historical_names": [],
  "admin_path": [
    {
      "id": 200,
      "type": "WARD",
      "name": "Phường A"
    },
    {
      "id": 1,
      "type": "CITY",
      "name": "Thành phố Hồ Chí Minh"
    }
  ],
  "location": {
    "lat": 10.0,
    "lon": 106.0
  },
  "popularity": 0.0,
  "source_quality": 0.0
}
```

### 9.5 Normalization

Tối thiểu cần chuẩn hóa:

- Unicode, case, whitespace và punctuation.
- Bản có dấu và không dấu.
- Viết tắt hành chính và đường phố.
- Alias và tên cũ.
- Typo/common variant theo dữ liệu thực tế.
- Historical/current administrative context.

### 9.6 Retrieval và ranking

``` text
Raw query
   ↓
Normalize
   ↓
Parse components
   ↓
Candidate retrieval
   ↓
Resolve admin/place context
   ↓
Historical resolution
   ↓
Geo validation / proximity
   ↓
Ranking / reranking
   ↓
Suggestions
```

Ranking có thể kết hợp text relevance, administrative context, alias/history match, geographic proximity, popularity, source/data quality và logistics signal.

## 10. Data Integrity và Constraints

### 10.1 Constraint bắt buộc

- Primary key cho mọi canonical table.
- Foreign key cho relationship nội bộ khi lifecycle cho phép.
- `NOT NULL` cho thuộc tính bắt buộc.
- `UNIQUE` cho business key thực sự duy nhất.
- `CHECK` cho temporal range, enum-like values và invariant đơn giản.
- Exclusion/temporal constraint cho khoảng hiệu lực không được overlap.
- Không cho phép hierarchy tự trỏ hoặc cycle.
- External identity unique theo `(source_id, external_type, external_id)`.
- `external_references` phải trỏ **chính xác một** canonical target.
- Alias duplicate phải được kiểm tra trên normalized value trong đúng scope.
- Change event phải có member hợp lệ theo loại change.

### 10.2 Temporal integrity

``` sql
CHECK (valid_to IS NULL OR valid_to > valid_from)
```

Với model `[valid_from, valid_to)`, dùng `>` để tránh zero-length period.

Không dùng riêng `UNIQUE(unit_code, valid_from)` làm temporal integrity. Dùng range + exclusion constraint hoặc temporal constraint phù hợp với PostgreSQL version.

### 10.3 Geographic integrity

- `geo_boundaries.boundary`: `MultiPolygon`, SRID 4326.
- `delivery_points.location`: `Point`, SRID 4326.
- Spatial geometry phải qua validation trước khi dùng cho reverse geocoding.
- Source/version phải truy vết được đối với boundary quan trọng.
- Khoảng cách theo mét phải dùng `geography` hoặc projected CRS phù hợp.

### 10.4 Delete policy

Canonical geographic/address entity ưu tiên soft lifecycle:

``` text
ACTIVE
INACTIVE
MERGED
SUPERSEDED
```

kết hợp temporal validity khi cần. Không mặc định hard delete dữ liệu đã tham gia history/provenance.

## 11. Indexing Strategy

Index chỉ được thêm khi phục vụ query pattern hoặc constraint cụ thể. Mọi index quan trọng cần được xác nhận bằng `EXPLAIN (ANALYZE, BUFFERS)` trên workload đại diện.

### 11.1 B-tree / relational lookup

``` sql
CREATE INDEX idx_admin_units_parent
ON administrative_units(parent_id);

CREATE INDEX idx_admin_units_name
ON administrative_units(normalized_name);

CREATE INDEX idx_admin_alias_normalized
ON administrative_unit_aliases(normalized_alias);

CREATE INDEX idx_places_parent
ON places(parent_place_id);

CREATE INDEX idx_places_type_status
ON places(place_type, status);

CREATE INDEX idx_place_alias_normalized
ON place_aliases(normalized_alias);

CREATE INDEX idx_place_admin_place
ON place_admin_relations(place_id);

CREATE INDEX idx_place_admin_unit
ON place_admin_relations(admin_unit_id);

CREATE UNIQUE INDEX uq_external_reference_source_object
ON external_references(source_id, external_type, external_id);
```

### 11.2 Spatial index

``` sql
CREATE INDEX idx_places_location_gist
ON places USING GIST(location);

CREATE INDEX idx_geo_boundaries_boundary_gist
ON geo_boundaries USING GIST(boundary);

CREATE INDEX idx_place_geometries_geometry_gist
ON place_geometries USING GIST(geometry);

CREATE INDEX idx_delivery_points_location_gist
ON delivery_points USING GIST(location);
```

Nếu workload dùng thường xuyên `location::geography`, cần benchmark và có thể dùng expression index:

``` sql
CREATE INDEX idx_places_location_geography_gist
ON places
USING GIST ((location::geography));
```

Chỉ thêm index khi query profile chứng minh có lợi. Không tạo index cho mọi cột.

## 12. Data Lifecycle, Sync và Rebuild

### 12.1 Ingestion lifecycle

``` text
External Source
   ↓
Raw / Staging
   ↓
Validate
   ↓
Normalize
   ↓
Match / Reconcile
   ↓
Canonical PostgreSQL / PostGIS
   ↓
Index Builder
   ↓
Elasticsearch
```

Tách raw/staging khỏi canonical schema để dữ liệu nguồn chưa xác minh không làm ô nhiễm master data.

### 12.2 Incremental sync

Không đồng bộ Elasticsearch row-to-row một cách mù quáng, vì một search document là aggregate/denormalized representation của nhiều bảng.

``` text
Business transaction
      │
      ├── update canonical tables
      └── insert outbox event
              │
              ▼
       Index Worker polling
              │
      load canonical aggregate
              │
              ▼
        Elasticsearch
```

Baseline MVP dùng index worker poll trực tiếp `outbox_events`; không cần broker, CDC hoặc `LISTEN/NOTIFY`.

Nếu workload tương lai chứng minh polling không đáp ứng SLO, Transactional Outbox và CDC có thể phối hợp:

- Outbox định nghĩa semantic event cần phát sinh cùng business transaction.
- CDC có thể đọc outbox hoặc WAL để publish event.
- Debezium là một implementation option cho PostgreSQL logical decoding/CDC.
- Index worker phải có retry, idempotency và recovery rõ ràng.

Ví dụ event:

``` json
{
  "event_type": "SEARCH_ENTITY_CHANGED",
  "entity_type": "PLACE",
  "entity_id": 100,
  "reason": "ADMIN_RELATION_UPDATED"
}
```

Indexer rebuild search-context document từ canonical state hiện tại, thay vì patch Elasticsearch chỉ từ row change thô.

### 12.3 Idempotency và ordering

Pipeline phải xử lý được:

- event delivery lặp;
- retry sau lỗi Elasticsearch;
- out-of-order event;
- entity bị merge/supersede;
- một canonical entity sinh nhiều search-context document.

Khuyến nghị mang theo version/revision đủ tin cậy để bỏ qua stale update.

### 12.4 Zero-downtime full reindex

``` text
places_autocomplete_v1
        ↑
places_autocomplete_active
```

Build:

``` text
places_autocomplete_v2
```

Validate xong thì swap alias:

``` text
places_autocomplete_active:
v1 → v2
```

Alias swap phải dùng một atomic alias operation. Chỉ xóa index cũ sau khi alias đúng, smoke test pass và rollback window đã được đáp ứng.

### 12.5 Rebuildability

Search index phải rebuild hoàn toàn từ canonical data + source metadata. Không để Elasticsearch chứa business truth duy nhất.

### 12.6 Indexing latency SLO

Không hard-code giả định như `< 1s` nếu chưa benchmark.

``` text
Target indexing lag: TBD by benchmark
```

Sau khi có workload thật mới chốt `p95`/`p99` và metric cho queue lag, outbox lag, indexing failures.

### 12.7 Provenance

Mỗi record quan trọng phải trả lời được:

``` text
Nguồn nào tạo ra dữ liệu?
External ID là gì?
Dataset/version nào?
Được import/sync khi nào?
Đã được xác minh hay chưa?
Canonical entity nào đang đại diện cho nó?
```

## 13. Quality Requirements

Các ngưỡng dưới đây là **mục tiêu thiết kế cần được benchmark/xác nhận**, không phải số liệu đã đo.

| ID    | Thuộc tính           | Mục tiêu/Acceptance Criteria                                                                                   |
|:------|:---------------------|:---------------------------------------------------------------------------------------------------------------|
| QR-01 | Correctness          | Canonical data không vi phạm PK/FK/temporal/spatial invariants đã định nghĩa                                   |
| QR-02 | Rebuildability       | Có thể tái tạo Elasticsearch từ PostgreSQL/PostGIS mà không cần dữ liệu bí mật ngoài canonical/source metadata |
| QR-03 | Traceability         | Entity từ nguồn ngoài truy ngược được `data_source` và `external_reference`                                    |
| QR-04 | Search quality       | Đánh giá bằng Top-1, Top-3/5, Recall@K, MRR/NDCG trên labeled query set                                        |
| QR-05 | Performance          | p50/p95/p99 autocomplete phải được benchmark trên workload đại diện trước khi chốt SLO                         |
| QR-06 | Maintainability      | Thêm nguồn dữ liệu mới không yêu cầu đổi primary key hoặc phá relationship canonical hiện hữu                  |
| QR-07 | Temporal correctness | Query theo thời điểm có thể phân biệt current/historical administrative context                                |

------------------------------------------------------------------------

## 14. Rủi ro và điểm cần xác nhận

| Rủi ro / Open Question                            | Ảnh hưởng                                             | Hướng xử lý                                                            |
|:--------------------------------------------------|:------------------------------------------------------|:-----------------------------------------------------------------------|
| Chất lượng/độ phủ OSM không đồng đều              | Geo/search quality khác nhau theo khu vực             | Kết hợp government + internal data, source priority và confidence      |
| Mapping OSM object vào canonical entity sai       | Duplicate hoặc merge nhầm Place                       | Matching pipeline + review/verification + external reference           |
| Boundary lịch sử thiếu                            | Reverse historical resolution không chính xác         | Version boundary theo nguồn; biểu diễn confidence/unknown rõ ràng      |
| Alias tăng quá nhanh                              | Search noise, index phình                             | Source, type, priority, verification và lifecycle                      |
| Delivery signal thiên lệch theo vùng có nhiều đơn | Ranking bias                                          | Chuẩn hóa theo geography/time và đánh giá theo segment                 |
| Eventual consistency PostgreSQL → Elasticsearch   | Search tạm thời cũ hơn canonical                      | Outbox polling + retry + idempotency + reindex + observability         |
| Một Place đi qua nhiều admin context              | Suggestion hiển thị hoặc geo context sai              | 1 canonical Place → N search-context documents                         |
| Temporal period overlap                           | Historical resolution trả nhiều version cùng hiệu lực | Range + exclusion/temporal constraint                                  |
| Polymorphic reference không có FK                 | Orphan provenance record                              | Explicit FK columns + exactly-one `CHECK`                              |
| Hard delete canonical entity                      | Mất provenance/history                                | Soft lifecycle + temporal validity; hạn chế cascade                    |
| Geography query dùng sai đơn vị                   | Radius/distance sai                                   | Cast `geography` hoặc transform projected CRS; benchmark index phù hợp |

## 15. Naming và Documentation Conventions

- Table/column/index: `snake_case`.
- Table dùng danh từ số nhiều: `places`, `data_sources`.
- Primary key: `id`.
- Foreign key: `<entity>_id`.
- Timestamp: hậu tố `_at`; business effective date: `valid_from`, `valid_to`, `effective_date`.
- Boolean: tên thể hiện trạng thái như `is_primary`, `verified`.
- Index: `idx_<table>_<purpose>`.
- Unique index/constraint: `uq_<table>_<purpose>`.
- Check constraint: `ck_<table>_<rule>`.
- Foreign key constraint: `fk_<table>_<referenced_table>`.
- Mọi bảng/column quan trọng nên có `COMMENT ON` trong migration/schema để documentation sống cùng database.
- Không dùng magic number cho domain value; ưu tiên lookup/check/enum strategy được thống nhất trong codebase.

------------------------------------------------------------------------

## 16. Data Dictionary Summary

| Table                           | Domain                 | Vai trò                                                    |
|:--------------------------------|:-----------------------|:-----------------------------------------------------------|
| `administrative_units`          | Administrative         | Đơn vị hành chính hiện tại/lịch sử                         |
| `administrative_unit_aliases`   | Administrative         | Tên cũ, viết tắt, tên thường gọi                           |
| `administrative_changes`        | Administrative History | Sự kiện rename/merge/split/boundary change                 |
| `administrative_change_members` | Administrative History | SOURCE/TARGET của change                                   |
| `places`                        | Place                  | Đường, hẻm, thôn/ấp, POI, building…                        |
| `place_aliases`                 | Place                  | Alias của place                                            |
| `place_admin_relations`         | Place                  | Quan hệ place ↔ administrative unit                        |
| `place_relations`               | Place                  | Quan hệ giữa các place                                     |
| `geo_boundaries`                | Geographic             | Boundary hành chính theo phiên bản                         |
| `place_geometries`              | Geographic             | Geometry đầy đủ của place                                  |
| `delivery_points`               | Geographic / Logistics | Entrance/delivery/pickup/access point đã tổng hợp/xác minh |
| `data_sources`                  | Governance             | Nguồn dataset và provenance                                |
| `external_references`           | Governance             | Mapping canonical entity ↔ external identity               |
| `outbox_events`                 | Operational            | Sự kiện durable cho transactional outbox polling           |
| `schema_migrations`             | Operational            | Phiên bản migration đã áp dụng                              |

------------------------------------------------------------------------

## 17. Kết luận

Core model được giữ ở ba lớp khái niệm rõ ràng:

``` text
Administrative Unit
        ≠
Place
        ≠
Search Document
```

PostgreSQL/PostGIS quản lý **canonical relational + spatial truth**; Elasticsearch quản lý **derived search representation**. OpenStreetMap là một nguồn dữ liệu geographic quan trọng nhưng không chi phối canonical schema. MVP chính thức theo hướng **place-centric**: không có canonical Address; số nhà và chuỗi địa chỉ được parser/resolver xử lý trên nền Administrative Unit + Place/Street + Geometry + Delivery Point.

Cấu trúc này ưu tiên tính đúng của dữ liệu, temporal history, provenance, khả năng rebuild search index và khả năng mở rộng sang geocoding/reverse geocoding/logistics intelligence mà không làm core schema phụ thuộc vào một nhà cung cấp dữ liệu hoặc một search engine.

------------------------------------------------------------------------

## 18. Quản lý phiên bản

Tài liệu sử dụng **Semantic Versioning (SemVer)**:

``` text
MAJOR.MINOR.PATCH
```

### 18.1 Quy ước

Trong giai đoạn `0.x`, thiết kế vẫn ở **initial development** và có thể có breaking change giữa các minor version.

| Thành phần         | Quy ước                                                                          |
|:-------------------|:---------------------------------------------------------------------------------|
| `0.MINOR.PATCH`    | Giai đoạn trước production contract; minor có thể chứa breaking design change    |
| `MAJOR` từ `1.0.0` | Breaking/incompatible change đối với production contract                         |
| `MINOR` từ `1.0.0` | Bổ sung backward-compatible                                                      |
| `PATCH`            | Sửa lỗi tài liệu, wording, format hoặc clarification không đổi semantic contract |

### 18.2 Version History

| Version | Ngày       | Trạng thái | Nội dung                                                                                                                                                             |
|:--------|:-----------|:-----------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `0.1.0` | 19/09/2026 | Baseline   | Baseline đầu tiên của Database Design Specification                                                                                                                  |
| `0.2.0` | 20/09/2026 | Superseded | Hoàn thiện temporal integrity, explicit FK cho external references, geometry/geography guidance, search-context indexing, sync strategy và chuẩn hóa Markdown tables |
| `0.3.0` | 20/09/2026 | Current    | Chốt place-centric MVP, Elasticsearch duy nhất, transactional outbox polling; Redis, broker, CDC và LISTEN/NOTIFY không thuộc baseline MVP                           |

> Khi schema và architectural contract được duyệt làm production baseline, promote lên `1.0.0`.

## 19. Tài liệu tham chiếu và nguyên tắc áp dụng

Tài liệu này được căn chỉnh theo các nguyên tắc phổ biến của tài liệu kiến trúc và thiết kế dữ liệu:

1.  **arc42** — tách rõ mục tiêu/phạm vi, constraints, solution decisions, quality requirements, risks và glossary/terminology; chỉ giữ các phần có giá trị cho Database Design Specification.
2.  **C4 Model** — diagram phải có scope rõ, không trộn mức abstraction, relationship có ý nghĩa và diagram có thể đọc độc lập.
3.  **PostgreSQL Documentation** — integrity được thể hiện bằng `NOT NULL`, `UNIQUE`, PK, FK, `CHECK` và các constraint phù hợp; schema/index phải phản ánh invariant thực.
4.  **PostgreSQL COMMENT** — mô tả table/column/constraint nên được lưu cùng database migration để giảm drift giữa tài liệu và schema.
5.  **Docs-as-Code** — Markdown được lưu cùng source control, review bằng pull request và cập nhật cùng migration/schema change.

### Reference links

- arc42: `https://arc42.org/overview/`
- arc42 Documentation: `https://docs.arc42.org/`
- C4 Model: `https://c4model.com/`
- C4 Diagram Review Checklist: `https://c4model.com/diagrams/checklist`
- PostgreSQL Constraints: `https://www.postgresql.org/docs/current/ddl-constraints.html`
- PostgreSQL COMMENT: `https://www.postgresql.org/docs/current/sql-comment.html`
