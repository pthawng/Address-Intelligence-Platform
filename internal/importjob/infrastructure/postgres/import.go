// Package postgres imports the checked-in SuperShip SQL snapshots without
// executing their obsolete admin_units INSERT statements.
package postgres

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"address-intelligence-platform/internal/platform/database"
	"github.com/jackc/pgx/v5"
)

type snapshot struct {
	code, file string
	count      int
}

var snapshots = []snapshot{
	{"SUPERSHIP_2_L1", "SuperShip - DỮ LIỆU 2 CẤP - LEVEL_1.sql", 34},
	{"SUPERSHIP_2_L3", "SuperShip - DỮ LIỆU 2 CẤP - LEVEL_3.sql", 3321},
	{"SUPERSHIP_2_L4", "SuperShip - DỮ LIỆU 2 CẤP - LEVEL_4.sql", 171291},
	{"SUPERSHIP_3_L1", "SuperShip - DỮ LIỆU 3 CẤP - LEVEL_1.sql", 63},
	{"SUPERSHIP_3_L2", "SuperShip - DỮ LIỆU 3 CẤP - LEVEL_2.sql", 709},
	{"SUPERSHIP_3_L3", "SuperShip - DỮ LIỆU 3 CẤP - LEVEL_3.sql", 11163},
}

type record struct {
	line, level, area                int
	code, name, parent, kind, status string
}

type input struct {
	snapshot snapshot
	hash     string
	rows     []record
}

// The source files contain one narrowly defined INSERT form per line. Parsing
// data rather than executing source SQL preserves the old source identity while
// preventing obsolete table names and conflict handlers from changing schema.
var insert = regexp.MustCompile(`^INSERT INTO admin_units \(unit_code, unit_name, unit_level(?:, parent_id)?, area_type\) VALUES \('((?:''|[^'])*)', '((?:''|[^'])*)', ([1-4])(?:, '((?:''|[^'])*)')?, ([12])\) ON CONFLICT \(unit_code\) DO (?:NOTHING|UPDATE SET .+);$`)

func parseLine(line string, number int) (record, error) {
	m := insert.FindStringSubmatch(line)
	if m == nil {
		return record{}, fmt.Errorf("unsupported input SQL at line %d", number)
	}
	r := record{line: number, code: strings.ReplaceAll(m[1], "''", "'"), name: strings.ReplaceAll(m[2], "''", "'"), level: int(m[3][0] - '0'), parent: strings.ReplaceAll(m[4], "''", "'"), area: int(m[5][0] - '0')}
	if (r.level == 1) != (r.parent == "") {
		return record{}, fmt.Errorf("invalid source hierarchy at line %d", number)
	}
	r.status = "STAGED"
	if r.code == "" || strings.TrimSpace(r.name) == "" {
		if r.level < 4 {
			return record{}, fmt.Errorf("invalid administrative unit at line %d", number)
		}
		r.status = "INVALID"
	}
	if r.level < 4 {
		var ok bool
		r.kind, ok = unitType(r.name)
		if !ok {
			return record{}, fmt.Errorf("unknown administrative unit type at line %d", number)
		}
	}
	return r, nil
}

func unitType(name string) (string, bool) {
	for _, item := range []struct{ prefix, kind string }{
		{"Thành phố ", "CITY"}, {"Tỉnh ", "PROVINCE"},
		{"Quận ", "DISTRICT"}, {"Huyện ", "DISTRICT"},
		{"Thị xã ", "TOWN"}, {"Thị trấn ", "TOWNSHIP"},
		{"Phường ", "WARD"}, {"Xã ", "COMMUNE"},
		{"Đặc khu ", "SPECIAL_ZONE"},
	} {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), strings.ToLower(item.prefix)) {
			return item.kind, true
		}
	}
	return "", false
}

func readInput(dir string, spec snapshot) (input, error) {
	f, err := os.Open(filepath.Join(dir, spec.file))
	if err != nil {
		return input{}, err
	}
	defer f.Close()
	h := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(f, h))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	in := input{snapshot: spec, rows: make([]record, 0, spec.count)}
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "--") {
			continue
		}
		r, err := parseLine(text, line)
		if err != nil {
			return input{}, fmt.Errorf("%s: %w", spec.file, err)
		}
		if (spec.code[10] == '2' && r.area != 2) || (spec.code[10] == '3' && r.area != 1) || r.level != int(spec.code[len(spec.code)-1]-'0') {
			return input{}, fmt.Errorf("%s: wrong level or area at line %d", spec.file, line)
		}
		in.rows = append(in.rows, r)
	}
	if err := scanner.Err(); err != nil {
		return input{}, err
	}
	if len(in.rows) != spec.count {
		return input{}, fmt.Errorf("%s: expected %d rows, got %d", spec.file, spec.count, len(in.rows))
	}
	in.hash = hex.EncodeToString(h.Sum(nil))
	return in, nil
}

func validate(inputs []input) error {
	admins := map[string]record{}
	for _, in := range inputs {
		for _, r := range in.rows {
			if r.level == 4 {
				continue
			}
			if _, exists := admins[r.code]; exists {
				return fmt.Errorf("duplicate administrative code %q", r.code)
			}
			admins[r.code] = r
		}
	}
	for _, r := range admins {
		if r.parent == "" {
			continue
		}
		parent, exists := admins[r.parent]
		if !exists || parent.area != r.area || parent.level >= r.level {
			return fmt.Errorf("missing or invalid parent for administrative code %q", r.code)
		}
	}
	return nil
}

// Import atomically stages every source row and promotes only levels 1-3.
// Level 4 mixes streets and POIs, has duplicate source codes, and usually lacks
// a resolvable administrative parent. Those records remain UNRESOLVED.
func Import(ctx context.Context, url, dir string) (int, bool, error) {
	inputs := make([]input, 0, len(snapshots))
	for _, spec := range snapshots {
		in, err := readInput(dir, spec)
		if err != nil {
			return 0, false, err
		}
		inputs = append(inputs, in)
	}
	if err := validate(inputs); err != nil {
		return 0, false, err
	}
	connection, err := pgx.ParseConfig(url)
	if err != nil {
		return 0, false, errors.New("invalid source import connection configuration")
	}
	connection.RuntimeParams["search_path"] = "public"
	connection.RuntimeParams["application_name"] = "address-intelligence-importdata"
	connection.RuntimeParams["lock_timeout"] = "5000"
	connection.RuntimeParams["statement_timeout"] = "600000"
	conn, err := pgx.ConnectConfig(ctx, connection)
	if err != nil {
		return 0, false, errors.New("source import database connection failed")
	}
	defer conn.Close(ctx)
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)
	var version int64
	if err := tx.QueryRow(ctx, `SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1`).Scan(&version); err != nil || version != database.SchemaVersion {
		return 0, false, errors.New("source import requires current schema version")
	}
	existing := 0
	for _, in := range inputs {
		var hash string
		var count, linked, held int
		err := tx.QueryRow(ctx, `SELECT coalesce(d.version,''), count(r.id),
			count(*) FILTER (WHERE r.raw_level<4 AND r.resolution_status='LINKED' AND r.canonical_admin_unit_id IS NOT NULL),
			count(*) FILTER (WHERE r.raw_level=4 AND r.resolution_status IN ('UNRESOLVED','INVALID'))
			FROM data_sources d LEFT JOIN source_records r ON r.source_id=d.id WHERE d.source_code=$1 GROUP BY d.id`, in.snapshot.code).Scan(&hash, &count, &linked, &held)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, false, err
		}
		wantLinked, wantHeld := 0, 0
		for _, r := range in.rows {
			if r.level < 4 {
				wantLinked++
			} else {
				wantHeld++
			}
		}
		if hash != in.hash || count != len(in.rows) || linked != wantLinked || held != wantHeld {
			return 0, false, fmt.Errorf("existing source %s differs from input snapshot", in.snapshot.code)
		}
		existing++
	}
	if existing == len(inputs) {
		return 0, true, nil
	}
	if existing != 0 {
		return 0, false, errors.New("partial source import exists; inspect before retry")
	}
	var overlap int
	adminCodes := make([]string, 0, 15290)
	for _, in := range inputs {
		for _, r := range in.rows {
			if r.level < 4 {
				adminCodes = append(adminCodes, r.code)
			}
		}
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM administrative_units WHERE unit_code = ANY($1)`, adminCodes).Scan(&overlap); err != nil {
		return 0, false, err
	}
	if overlap != 0 {
		return 0, false, fmt.Errorf("%d administrative codes already exist; import requires reconciliation", overlap)
	}
	ids := make([]int64, 0, len(inputs))
	for _, in := range inputs {
		var id int64
		err := tx.QueryRow(ctx, `INSERT INTO data_sources(source_code,source_name,source_type,reference,version,imported_at,metadata) VALUES($1,$2,'INTERNAL',$3,$4,clock_timestamp(),jsonb_build_object('input_format','legacy_admin_units_sql')) RETURNING id`, in.snapshot.code, in.snapshot.file, in.snapshot.file, in.hash).Scan(&id)
		if err != nil {
			return 0, false, err
		}
		ids = append(ids, id)
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"source_records"}, []string{"source_id", "source_line", "external_id", "raw_name", "raw_level", "raw_parent_id", "area_type", "unit_type", "resolution_status"}, pgx.CopyFromSlice(len(in.rows), func(i int) ([]any, error) {
			r := in.rows[i]
			var parent, kind any
			if r.parent != "" {
				parent = r.parent
			}
			if r.kind != "" {
				kind = r.kind
			}
			return []any{id, r.line, r.code, r.name, r.level, parent, r.area, kind, r.status}, nil
		}))
		if err != nil {
			return 0, false, fmt.Errorf("stage %s: %w", in.snapshot.code, err)
		}
	}
	for _, level := range []int{1, 2, 3} {
		result, err := tx.Exec(ctx, `INSERT INTO administrative_units(unit_code,unit_name,normalized_name,unit_type,admin_level,parent_id,status,source_id)
			SELECT r.external_id,trim(r.raw_name),lower(regexp_replace(trim(r.raw_name),'\s+',' ','g')),r.unit_type,r.raw_level,p.id,
			CASE WHEN r.area_type=2 THEN 'ACTIVE' ELSE 'INACTIVE' END,r.source_id
			FROM source_records r LEFT JOIN administrative_units p ON p.unit_code=r.raw_parent_id
			WHERE r.source_id=ANY($1) AND r.raw_level=$2 ORDER BY r.external_id`, ids, level)
		if err != nil {
			return 0, false, fmt.Errorf("promote level %d: %w", level, err)
		}
		if result.RowsAffected() == 0 {
			return 0, false, fmt.Errorf("no administrative units at level %d", level)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE source_records r SET canonical_admin_unit_id=u.id,resolution_status='LINKED' FROM administrative_units u WHERE r.source_id=ANY($1) AND r.raw_level<4 AND u.unit_code=r.external_id`, ids)
	if err != nil {
		return 0, false, err
	}
	_, err = tx.Exec(ctx, `UPDATE source_records SET resolution_status='UNRESOLVED' WHERE source_id=ANY($1) AND raw_level=4 AND resolution_status='STAGED'`, ids)
	if err != nil {
		return 0, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO external_references(source_id,admin_unit_id,external_type,external_id,external_version)
		SELECT r.source_id,r.canonical_admin_unit_id,'ADMIN_UNIT',r.external_id,d.version FROM source_records r JOIN data_sources d ON d.id=r.source_id WHERE r.source_id=ANY($1) AND r.raw_level<4`, ids)
	if err != nil {
		return 0, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,aggregate_revision,payload)
		SELECT 'administrative_unit.upserted','ADMINISTRATIVE_UNIT',r.canonical_admin_unit_id,1,jsonb_build_object('unit_code',r.external_id)
		FROM source_records r WHERE r.source_id=ANY($1) AND r.raw_level<4`, ids)
	if err != nil {
		return 0, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, err
	}
	return len(adminCodes), false, nil
}
