# ADDRESS INTELLIGENCE PLATFORM
## TỔNG QUAN DỰ ÁN

| Thông tin | Nội dung |
|---|---|
| **Tên dự án** | Address Intelligence Platform |
| **Tên tài liệu** | Tổng quan dự án |
| **Tác giả** | Lê Phước Thắng |
| **Phiên bản** | `v2.3.0` |
| **Trạng thái** | Baseline MVP |
| **Ngày cập nhật** | 20/09/2026 |

> **Quy ước phiên bản:** `MAJOR.MINOR.PATCH`  
> `MAJOR`: thay đổi lớn về định hướng hoặc phạm vi.  
> `MINOR`: bổ sung hoặc tổ chức lại nội dung đáng kể.  
> `PATCH`: chỉnh sửa nhỏ, câu chữ hoặc định dạng.

---

# 1. Mục đích tài liệu

Tài liệu này cung cấp cái nhìn tổng quan về dự án **Address Intelligence Platform**, giúp thống nhất:

| Nội dung | Mục đích |
|---|---|
| Bối cảnh | Xác định vấn đề thực tế cần giải quyết |
| Mục tiêu | Xác định kết quả dự án hướng tới |
| Đối tượng sử dụng | Xác định ai hoặc hệ thống nào sử dụng nền tảng |
| Phạm vi | Xác định dự án làm gì và không làm gì |
| Năng lực chính | Xác định các nhóm chức năng cốt lõi |
| Dữ liệu | Xác định các khái niệm dữ liệu quan trọng |
| Chất lượng | Xác định các khía cạnh cần đo lường |
| Ràng buộc | Xác định các điều kiện cần lưu ý khi thiết kế |
| Lộ trình | Xác định thứ tự triển khai |

Tài liệu này là đầu vào cho các tài liệu tiếp theo như:

1. Đặc tả yêu cầu phần mềm.
2. Thiết kế kiến trúc tổng thể.
3. Thiết kế cơ sở dữ liệu.
4. Đặc tả API.
5. Thiết kế chi tiết.
6. Đặc tả kiểm thử.
7. Các quyết định kiến trúc.

---

# 2. Tổng quan dự án

## 2.1. Mô tả

**Address Intelligence Platform** là nền tảng quản lý, tìm kiếm, chuẩn hóa và định vị dữ liệu địa chỉ Việt Nam.

Nền tảng sử dụng **bộ dữ liệu địa chỉ do hệ thống tự quản lý**, được cập nhật thường xuyên để phản ánh các thay đổi về địa chỉ và đơn vị hành chính.

## 2.2. Định hướng

| Định hướng | Mô tả |
|---|---|
| **Dữ liệu làm trung tâm** | Hệ thống chủ động quản lý bộ dữ liệu địa chỉ của riêng mình |
| **Việt Nam là phạm vi chính** | Tập trung xử lý đặc thù địa chỉ Việt Nam |
| **Hỗ trợ dữ liệu thay đổi theo thời gian** | Không ghi đè làm mất thông tin lịch sử |
| **Tìm kiếm linh hoạt** | Hỗ trợ có dấu, không dấu, viết tắt, tên cũ và dữ liệu chưa đầy đủ |
| **Định vị nhất quán** | Liên kết địa chỉ với tọa độ và vị trí thực tế |
| **Có thể dùng chung** | Nhiều ứng dụng và hệ thống có thể sử dụng cùng một nguồn dữ liệu |

## 2.3. Phạm vi địa lý

| Nội dung | Quyết định |
|---|---|
| Phạm vi hiện tại | Việt Nam |
| Đa quốc gia | Không thuộc mục tiêu hiện tại |
| Khả năng mở rộng | Không chủ động khóa khả năng mở rộng trong tương lai |

---

# 3. Bối cảnh và vấn đề cần giải quyết

| Mã | Vấn đề | Mô tả |
|---|---|---|
| **P01** | Một địa chỉ có nhiều cách viết | Có dấu, không dấu, viết tắt, sai khác chính tả, tên thường gọi |
| **P02** | Địa chỉ thay đổi theo thời gian | Đơn vị hành chính có thể được thành lập mới, đổi tên, sáp nhập, chia tách, điều chỉnh địa giới, chuyển loại hoặc giải thể |
| **P03** | Tên địa chỉ không phải định danh ổn định | Tên có thể đổi nhưng vị trí thực tế vẫn giữ nguyên |
| **P04** | Dữ liệu đầu vào thường không đầy đủ | Người dùng có thể chỉ nhập số nhà, tên đường, tòa nhà hoặc một phần địa chỉ |
| **P05** | Dữ liệu cũ vẫn tiếp tục tồn tại | Hồ sơ, đơn hàng và hệ thống cũ có thể chứa tên địa chỉ trước thay đổi |
| **P06** | Không tìm thấy chưa chắc là không tồn tại | Có thể do bộ dữ liệu chưa cập nhật hoặc dữ liệu đầu vào chưa đủ |
| **P07** | Dữ liệu cần được cập nhật có kiểm soát | Cần biết dữ liệu nào thay đổi, thay đổi khi nào và có thể khôi phục khi lỗi |
| **P08** | Khó đánh giá chất lượng chỉ bằng trạng thái API | `200 OK` không đồng nghĩa địa chỉ hoặc tọa độ là chính xác |

Ví dụ nhiều cách biểu diễn cùng một địa chỉ:

```text
123 nguyen trai
123 Nguyễn Trãi
123 Nguyen Trai
123 Nguyễn Trãi, phường ...
```

Dự án cần giải quyết bài toán:

```text
Nhiều cách biểu diễn
        ↓
Xác định đúng đối tượng địa chỉ / vị trí
        ↓
Trả về dữ liệu chuẩn và nhất quán
```

---

# 4. Mục tiêu dự án

## 4.1. Mục tiêu tổng quát

Xây dựng một nguồn dữ liệu địa chỉ thống nhất cho Việt Nam, cho phép các hệ thống **tìm kiếm, xác định, chuẩn hóa và định vị địa chỉ một cách nhất quán**, đồng thời duy trì được lịch sử và chất lượng dữ liệu khi địa chỉ hoặc đơn vị hành chính thay đổi theo thời gian.

## 4.2. Mục tiêu cụ thể

| Mã | Mục tiêu | Kết quả mong muốn |
|---|---|---|
| **G01** | Xây dựng nguồn dữ liệu địa chỉ thống nhất | Các hệ thống sử dụng chung một bộ dữ liệu có cấu trúc |
| **G02** | Hỗ trợ tìm và xác định địa chỉ linh hoạt | Tìm được địa chỉ từ nhiều kiểu nhập khác nhau |
| **G03** | Duy trì lịch sử thay đổi | Có thể tra cứu và liên kết dữ liệu cũ với dữ liệu hiện tại |
| **G04** | Cung cấp thông tin vị trí đáng tin cậy | Có thể chuyển đổi giữa địa chỉ và tọa độ với thông tin về độ chính xác |
| **G05** | Quản lý vòng đời dữ liệu có kiểm soát | Dữ liệu được kiểm tra, quản lý phiên bản, xuất bản và khôi phục |
| **G06** | Đo lường chất lượng dữ liệu và kết quả | Biết mức độ đầy đủ, mới, chính xác và khả năng tìm kiếm của dữ liệu |
| **G07** | Phản ánh thay đổi dữ liệu nhanh chóng tới chức năng tra cứu | Sau khi dữ liệu hành chính được duyệt và xuất bản, nội dung tìm kiếm và gợi ý được cập nhật gần như tức thời theo mục tiêu thời gian được quy định |

---

# 5. Đối tượng sử dụng

| Nhóm | Ví dụ | Nhu cầu chính |
|---|---|---|
| **Người dùng cuối** | Người dùng website, mobile app | Nhập và chọn đúng địa chỉ nhanh chóng |
| **Hệ thống nghiệp vụ** | Đơn hàng, giao vận, CRM, khách hàng | Sử dụng địa chỉ chuẩn và nhất quán |
| **Hệ thống dữ liệu** | Báo cáo, phân tích | Sử dụng dữ liệu địa chỉ có cấu trúc |
| **Đội quản trị dữ liệu** | Data Admin / Operations | Cập nhật, kiểm tra, xuất bản và sửa dữ liệu |
| **Đội phát triển** | Backend, Mobile, Web | Sử dụng API chung thay vì tự xử lý địa chỉ |

---

# 6. Phạm vi dự án

Toàn bộ các nội dung dưới đây đều thuộc phạm vi dự án. Việc chia giai đoạn chỉ thể hiện **thứ tự triển khai**, không phải phạm vi khác nhau.

| Nhóm năng lực | Nội dung chính |
|---|---|
| **Tra cứu địa chỉ** | Gợi ý địa chỉ, tìm kiếm, xếp hạng kết quả |
| **Chuẩn hóa và xác định địa chỉ** | Chuẩn hóa, xác định địa chỉ phù hợp, kiểm tra dữ liệu |
| **Xử lý cách nhập thực tế** | Có dấu, không dấu, viết tắt, sai khác chính tả, tên cũ, bí danh |
| **Dữ liệu hành chính** | Quản lý đơn vị hành chính và quan hệ thay đổi theo thời gian |
| **Phân giải địa chỉ** | Phân tích chuỗi nhập thành số nhà, đường/địa điểm và ngữ cảnh hành chính; kết quả không có định danh canonical bền vững trong MVP |
| **Địa điểm và điểm giao nhận** | Tòa nhà, địa danh, cổng, lối vào, điểm giao nhận |
| **Định vị** | Địa chỉ → tọa độ, tọa độ → địa chỉ, tìm kiếm theo vị trí |
| **Quản lý dữ liệu** | Nhập, kiểm tra, quản lý phiên bản, xuất bản, khôi phục |
| **Quản lý tìm kiếm** | Chỉ mục Elasticsearch, đồng bộ dữ liệu gần thời gian thực bằng transactional outbox polling và cập nhật gợi ý |
| **Quản lý chất lượng** | Theo dõi độ đầy đủ, độ mới, độ chính xác và khả năng tìm kiếm |
| **Vận hành** | Giám sát, nhật ký thay đổi, quản lý quyền, trang quản trị |

---

# 7. Các trường hợp sử dụng chính

| Mã | Trường hợp sử dụng | Mô tả ngắn |
|---|---|---|
| **UC01** | Gợi ý địa chỉ | Trả gợi ý phù hợp trong khi người dùng đang nhập |
| **UC02** | Tìm kiếm địa chỉ | Tìm địa chỉ từ chuỗi đầy đủ hoặc gần đầy đủ |
| **UC03** | Tìm bằng tên cũ / địa chỉ cũ | Vẫn tìm được dữ liệu dù người dùng sử dụng tên hành chính đã hết hiệu lực |
| **UC04** | Chuẩn hóa địa chỉ | Chuyển các cách viết khác nhau về biểu diễn chuẩn |
| **UC05** | Xác định địa chỉ | Xác định địa chỉ phù hợp từ dữ liệu không đầy đủ hoặc không đồng nhất |
| **UC06** | Địa chỉ → tọa độ | Trả vị trí địa lý của địa chỉ |
| **UC07** | Tọa độ → địa chỉ | Tìm địa chỉ, địa điểm hoặc đơn vị hành chính tương ứng với tọa độ |
| **UC08** | Thành lập đơn vị hành chính mới | Ghi nhận đơn vị mới, thời điểm hiệu lực và quan hệ với dữ liệu liên quan |
| **UC09** | Đổi tên đơn vị hành chính | Duy trì định danh, lịch sử tên cũ và tên mới để cả hai vẫn có thể tra cứu theo quy tắc |
| **UC10** | Sáp nhập đơn vị hành chính | Ghi nhận nhiều đơn vị cũ được hợp nhất vào một đơn vị mới và duy trì quan hệ lịch sử |
| **UC11** | Chia tách đơn vị hành chính | Ghi nhận một đơn vị cũ được tách thành nhiều đơn vị mới và quản lý quan hệ kế thừa |
| **UC12** | Điều chỉnh địa giới hành chính | Cập nhật phạm vi đơn vị và các đối tượng địa chỉ bị ảnh hưởng mà không làm mất lịch sử trước thay đổi |
| **UC13** | Chuyển loại đơn vị hành chính | Ghi nhận việc thay đổi loại đơn vị, ví dụ từ xã sang phường, theo thời điểm hiệu lực |
| **UC14** | Giải thể / chấm dứt hiệu lực đơn vị hành chính | Đánh dấu đơn vị không còn hiệu lực và liên kết tới đơn vị kế thừa khi có |
| **UC15** | Cập nhật bộ dữ liệu mới | Nhập, kiểm tra, so sánh và xuất bản phiên bản dữ liệu mới |
| **UC16** | Đồng bộ thay đổi tới tìm kiếm gần thời gian thực | Sau khi thay đổi được xuất bản, cập nhật chỉ mục, gợi ý và dữ liệu tra cứu với độ trễ rất thấp |
| **UC17** | Tra cứu trong thời gian chuyển đổi dữ liệu | Trong lúc cập nhật, người dùng vẫn nhận kết quả nhất quán theo phiên bản dữ liệu đang có hiệu lực |
| **UC18** | Khôi phục phiên bản dữ liệu | Khi bản cập nhật có lỗi, quay lại phiên bản ổn định trước đó và đồng bộ lại dữ liệu phục vụ tìm kiếm |
| **UC19** | Theo dõi lịch sử thay đổi | Tra cứu một đơn vị / địa chỉ đã thay đổi như thế nào qua các phiên bản |
| **UC20** | Kiểm tra tác động của thay đổi hành chính | Xác định các địa chỉ, địa điểm, tọa độ hoặc bản ghi tìm kiếm bị ảnh hưởng trước khi xuất bản |

Ví dụ trạng thái khi xác định địa chỉ:

```text
MATCHED
PARTIAL_MATCH
AMBIGUOUS
NOT_FOUND
OUTDATED_REFERENCE
UNVERIFIED
```

Tên trạng thái chính thức sẽ được xác định trong tài liệu đặc tả yêu cầu.

## 7.1. Luồng cập nhật thay đổi hành chính

Ở mức tổng quan, các thay đổi như **thành lập mới, đổi tên, sáp nhập, chia tách, điều chỉnh địa giới, chuyển loại hoặc giải thể** được xử lý theo luồng:

```text
Tiếp nhận thay đổi
      ↓
Kiểm tra và đối chiếu
      ↓
Xác định dữ liệu bị ảnh hưởng
      ↓
Tạo phiên bản dữ liệu mới
      ↓
Duyệt / xuất bản
      ↓
Cập nhật dữ liệu phục vụ tra cứu
      ↓
Cập nhật chỉ mục tìm kiếm và gợi ý
      ↓
Làm mới / vô hiệu bộ nhớ đệm liên quan
      ↓
Theo dõi chất lượng sau cập nhật
```

Sau khi phiên bản mới được **xuất bản**, thay đổi phải được phản ánh tới chức năng tìm kiếm và gợi ý theo cơ chế **gần thời gian thực**.

Giá trị cụ thể như “tối đa bao nhiêu giây từ lúc xuất bản đến khi tìm kiếm nhìn thấy dữ liệu mới” sẽ được xác định thành yêu cầu đo lường được trong tài liệu đặc tả yêu cầu phần mềm.

---

# 8. Dữ liệu và nguyên tắc quản lý

## 8.1. Các khái niệm dữ liệu chính

| Khái niệm | Mô tả |
|---|---|
| **Đơn vị hành chính** | Đơn vị hành chính có định danh, tên, loại và thời gian hiệu lực |
| **Kết quả phân giải địa chỉ** | Biểu diễn được tạo khi xử lý truy vấn, gồm số nhà và các tham chiếu tới Place/Street, đơn vị hành chính, geometry hoặc delivery point; không phải canonical entity và không có ID bền vững trong MVP |
| **Thành phần truy vấn địa chỉ** | Số nhà và các token/ngữ cảnh do parser/resolver trích xuất; không được materialize thành canonical table |
| **Bí danh** | Tên viết tắt, không dấu, tên cũ hoặc cách gọi khác |
| **Địa điểm** | Tòa nhà, bệnh viện, trường học, trung tâm thương mại, địa danh... |
| **Điểm giao nhận** | Cổng, lối vào, sảnh, khu nhận hàng... |
| **Tọa độ** | Vị trí không gian kèm loại và mức độ chính xác |
| **Bộ dữ liệu** | Tập dữ liệu địa chỉ được quản lý theo phiên bản |

## 8.2. Nguyên tắc dữ liệu

| Nguyên tắc | Mô tả |
|---|---|
| **Không dùng tên làm định danh** | Tên có thể thay đổi theo thời gian |
| **Không ghi đè làm mất lịch sử** | Dữ liệu cũ cần được lưu và liên kết với dữ liệu mới |
| **Không cố định số cấp hành chính** | Mô hình phải thích ứng khi cấu trúc hành chính thay đổi |
| **Không cấp identity cho địa chỉ resolve** | Canonical identity chỉ thuộc Administrative Unit, Place/Street, Geometry và Delivery Point; chuỗi địa chỉ và số nhà là đầu vào/kết quả của parser/resolver |
| **Phân biệt không tìm thấy và không hợp lệ** | Không tìm thấy trong dữ liệu chưa đủ để kết luận địa chỉ sai |
| **Có nguồn gốc và phiên bản** | Dữ liệu cần biết đến từ đâu và thuộc phiên bản nào |

## 8.3. Vòng đời dữ liệu

Ở mức tổng quan, dữ liệu được quản lý theo chu trình:

```text
Tiếp nhận
   ↓
Kiểm tra
   ↓
So sánh và phân tích tác động
   ↓
Quản lý phiên bản
   ↓
Xuất bản
   ↓
Đồng bộ tìm kiếm gần thời gian thực
   ↓
Theo dõi chất lượng
```

Chi tiết trạng thái và quy trình xử lý sẽ được mô tả ở tài liệu yêu cầu và thiết kế.

---

# 9. Chất lượng và vận hành

## 9.1. Các nhóm chất lượng cần theo dõi

| Nhóm | Ví dụ |
|---|---|
| **Chất lượng dữ liệu** | Thiếu dữ liệu, trùng dữ liệu, quan hệ hành chính sai |
| **Độ mới dữ liệu** | Thời điểm cập nhật, phiên bản hiện hành |
| **Chất lượng tìm kiếm** | Tỷ lệ không có kết quả, kết quả không rõ ràng |
| **Chất lượng xác định địa chỉ** | Tỷ lệ khớp, khớp một phần, chưa xác minh |
| **Chất lượng tọa độ** | Tỷ lệ có tọa độ, mức độ chính xác |
| **Dữ liệu lịch sử** | Tỷ lệ địa chỉ cũ được liên kết với dữ liệu hiện tại |
| **Độ trễ cập nhật tìm kiếm** | Thời gian từ lúc phiên bản dữ liệu được xuất bản đến khi thay đổi xuất hiện trong tìm kiếm / gợi ý |

## 9.2. Các thông tin vận hành cần theo dõi

- Số lượng yêu cầu.
- Thời gian phản hồi.
- Tỷ lệ lỗi.
- Tỷ lệ dùng bộ nhớ đệm.
- Trạng thái chỉ mục tìm kiếm.
- Phiên bản dữ liệu đang hoạt động.
- Trạng thái cập nhật dữ liệu.
- Độ trễ đồng bộ từ dữ liệu đã xuất bản sang chỉ mục tìm kiếm.
- Số bản ghi chưa đồng bộ hoặc đồng bộ lỗi.
- Lỗi trong quá trình nhập hoặc xuất bản dữ liệu.

## 9.3. Trang quản trị

Trang quản trị dự kiến hỗ trợ:

- Xem phiên bản dữ liệu.
- Nhập dữ liệu mới.
- Xem kết quả kiểm tra dữ liệu.
- So sánh thay đổi.
- Xuất bản dữ liệu.
- Khôi phục phiên bản.
- Tra cứu lịch sử.
- Quản lý bí danh.
- Theo dõi chất lượng.
- Xem nhật ký thay đổi.

---

# 10. Ranh giới, giả định và ràng buộc

## 10.1. Ngoài phạm vi

| Nội dung | Trạng thái |
|---|---|
| Chỉ đường từng chặng | Ngoài phạm vi |
| Tối ưu tuyến đường | Ngoài phạm vi |
| Tối ưu đội xe | Ngoài phạm vi |
| Dự đoán giao thông | Ngoài phạm vi |
| Tính thời gian đến dự kiến | Ngoài phạm vi |
| Công cụ hiển thị bản đồ hoàn chỉnh | Ngoài phạm vi |
| Hệ thống điều phối giao vận | Ngoài phạm vi |
| Hệ thống quản lý vận tải | Ngoài phạm vi |

## 10.2. Giả định ban đầu

| Mã | Giả định |
|---|---|
| **A01** | Việt Nam là phạm vi dữ liệu hiện tại |
| **A02** | Bộ dữ liệu địa chỉ do hệ thống tự quản lý |
| **A03** | Dữ liệu được cập nhật định kỳ |
| **A04** | Các hệ thống khác truy cập thông qua API |
| **A05** | Gợi ý địa chỉ cần thời gian phản hồi thấp |
| **A06** | Tìm kiếm cần hỗ trợ tiếng Việt có dấu và không dấu |
| **A07** | Dữ liệu lịch sử phải có thể tra cứu |
| **A08** | Một địa chỉ có thể có nhiều cách biểu diễn |
| **A09** | Bộ dữ liệu cần được quản lý phiên bản |
| **A10** | Hệ thống cần có khả năng khôi phục dữ liệu khi cập nhật lỗi |
| **A11** | Thay đổi đã được xuất bản cần được phản ánh tới tìm kiếm và gợi ý gần thời gian thực |
| **A12** | MVP theo mô hình place-centric; không có canonical Address hoặc bảng `addresses` |
| **A13** | Elasticsearch là search engine duy nhất của MVP; Redis chỉ được xem xét sau khi benchmark chứng minh cache có giá trị |

## 10.3. Ràng buộc cần xem xét

| Nhóm | Nội dung |
|---|---|
| **Dữ liệu** | Quy mô, độ chính xác, độ mới, tần suất cập nhật |
| **Tìm kiếm** | Khả năng xử lý tiếng Việt, tốc độ xây dựng chỉ mục |
| **Nhất quán** | Đồng bộ giữa cơ sở dữ liệu và chỉ mục tìm kiếm |
| **Hiệu năng** | Thời gian phản hồi và độ sẵn sàng |
| **Lịch sử** | Dung lượng lưu dữ liệu cũ và quan hệ thay đổi |
| **Vận hành** | Quyền thay đổi, duyệt và xuất bản dữ liệu |
| **Cập nhật** | Hạn chế gián đoạn dịch vụ khi thay dữ liệu mới |
| **Đồng bộ tìm kiếm** | Cần kiểm soát độ trễ, thứ tự và tính nhất quán khi cập nhật chỉ mục gần thời gian thực |

---

# 11. Kế hoạch triển khai và tài liệu tiếp theo

## 11.1. Thứ tự triển khai dự kiến

| Giai đoạn | Trọng tâm |
|---|---|
| **Giai đoạn 1 (MVP)** | Administrative Unit, Place/Street, Geometry, Delivery Point, Elasticsearch autocomplete/search và transactional outbox polling |
| **Giai đoạn 2** | Chuẩn hóa, xác định địa chỉ, bí danh, dữ liệu lịch sử, quản lý các loại thay đổi hành chính và phiên bản dữ liệu |
| **Giai đoạn 3** | Định vị, tọa độ, điểm giao nhận, phân tích tác động, quản trị và chất lượng nâng cao |

Các giai đoạn trên chỉ thể hiện **thứ tự triển khai**. Toàn bộ năng lực đã nêu vẫn thuộc phạm vi của dự án.

## 11.2. Nguyên tắc phân rã yêu cầu

```text
Vấn đề
    ↓
Mục tiêu
    ↓
Tính năng
    ↓
Câu chuyện người dùng
    ↓
Quy tắc nghiệp vụ
    ↓
Tiêu chí chấp nhận
    ↓
Trường hợp kiểm thử
```

## 11.3. Các tài liệu tiếp theo

| Thứ tự | Tài liệu |
|---|---|
| **01** | Tổng quan dự án |
| **02** | Đặc tả yêu cầu phần mềm |
| **03** | Thiết kế kiến trúc tổng thể |
| **04** | Thiết kế cơ sở dữ liệu |
| **05** | Đặc tả API |
| **06** | Thiết kế chi tiết |
| **07** | Đặc tả kiểm thử |
| **08** | Các quyết định kiến trúc |

---

> **Tóm tắt:** Address Intelligence Platform là nền tảng place-centric cho Việt Nam. MVP quản lý canonical Administrative Unit, Place/Street, Geometry và Delivery Point; địa chỉ là kết quả phân giải không có identity bền vững. PostgreSQL/PostGIS là source of truth, Elasticsearch là search read model và thay đổi được đồng bộ bằng transactional outbox polling.
