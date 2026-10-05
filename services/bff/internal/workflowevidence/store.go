package workflowevidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrMissing = errors.New("workflow evidence not found")
var ErrConflict = errors.New("workflow evidence integrity conflict")

// Store is an internal storage primitive. Its caller owns live authorization,
// capture of authoritative values, the outer transaction and its commit.
type Store struct{}

func nilPort(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func storePort(ctx context.Context, tx pgx.Tx) error {
	if nilPort(ctx) || nilPort(tx) {
		return ErrInvalid
	}
	return ctx.Err()
}

func storeScope(appID string, hash [32]byte) error {
	if !validID(appID) || hash == ([32]byte{}) {
		return ErrInvalid
	}
	return nil
}

func selectedIDs(ids []string) ([]string, error) {
	if len(ids) > MaxFields {
		return nil, ErrTooLarge
	}
	out := append([]string{}, ids...)
	sort.Strings(out)
	for i, id := range out {
		if !validID(id) || i > 0 && id == out[i-1] {
			return nil, ErrInvalid
		}
	}
	return out, nil
}

// All declared byte contributions are checked before any hash or format check.
func incomingBundle(input Bundle) (Manifest, Bundle, error) {
	if len(input.Manifest) > MaxManifestBytes || len(input.Fields) > MaxFields {
		return Manifest{}, Bundle{}, ErrTooLarge
	}
	remaining := MaxBundleBytes - len(input.Manifest)
	for _, blob := range input.Fields {
		if len(blob.Body) > MaxFieldBytes || len(blob.Body) > remaining {
			return Manifest{}, Bundle{}, ErrTooLarge
		}
		remaining -= len(blob.Body)
	}
	owned := Bundle{Manifest: bytes.Clone(input.Manifest), Hash: input.Hash}
	if owned.Hash == ([32]byte{}) || sha256.Sum256(owned.Manifest) != owned.Hash {
		return Manifest{}, Bundle{}, ErrInvalid
	}
	manifest, err := DecodeManifest(owned.Manifest)
	if err != nil {
		if errors.Is(err, ErrTooLarge) {
			return Manifest{}, Bundle{}, ErrTooLarge
		}
		return Manifest{}, Bundle{}, ErrInvalid
	}
	if len(input.Fields) != len(manifest.Fields) {
		return Manifest{}, Bundle{}, ErrInvalid
	}
	incoming := make(map[string]Blob, len(input.Fields))
	for _, blob := range input.Fields {
		if !validID(blob.FieldID) {
			return Manifest{}, Bundle{}, ErrInvalid
		}
		if _, duplicate := incoming[blob.FieldID]; duplicate {
			return Manifest{}, Bundle{}, ErrInvalid
		}
		blob.Body = bytes.Clone(blob.Body)
		if sha256.Sum256(blob.Body) != blob.Hash {
			return Manifest{}, Bundle{}, ErrInvalid
		}
		field, err := DecodeField(blob.Body)
		if err != nil {
			if errors.Is(err, ErrTooLarge) {
				return Manifest{}, Bundle{}, ErrTooLarge
			}
			return Manifest{}, Bundle{}, ErrInvalid
		}
		if field.Definition.ID != blob.FieldID {
			return Manifest{}, Bundle{}, ErrInvalid
		}
		incoming[blob.FieldID] = blob
	}
	owned.Fields = make([]Blob, 0, len(manifest.Fields))
	for _, ref := range manifest.Fields {
		blob, ok := incoming[ref.FieldID]
		if !ok || blob.Hash != ref.Hash {
			return Manifest{}, Bundle{}, ErrInvalid
		}
		owned.Fields = append(owned.Fields, blob)
	}
	return manifest, owned, nil
}

const documentSQL = `
SELECT body, app_id::text, table_id::text, view_id::text, record_id::text,
 created_by::text, schema_version, record_version, field_count
FROM applications.workflow_evidence_documents
WHERE app_id=$1::uuid AND evidence_hash=$2
`

func loadManifest(ctx context.Context, tx pgx.Tx, appID string, hash [32]byte) (Manifest, []byte, error) {
	var body []byte
	var metadata Header
	var count int
	err := tx.QueryRow(ctx, documentSQL, appID, hash[:]).Scan(&body, &metadata.AppID,
		&metadata.TableID, &metadata.ViewID, &metadata.RecordID, &metadata.CreatedBy,
		&metadata.SchemaVersion, &metadata.RecordVersion, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return Manifest{}, nil, ErrMissing
	}
	if err != nil {
		return Manifest{}, nil, err
	}
	if sha256.Sum256(body) != hash {
		return Manifest{}, nil, ErrConflict
	}
	manifest, err := DecodeManifest(body)
	if err != nil {
		return Manifest{}, nil, ErrConflict
	}
	header := manifest.Header
	if header.AppID != appID || header.AppID != metadata.AppID ||
		header.TableID != metadata.TableID || header.ViewID != metadata.ViewID ||
		header.RecordID != metadata.RecordID || header.CreatedBy != metadata.CreatedBy ||
		header.SchemaVersion != metadata.SchemaVersion || header.RecordVersion != metadata.RecordVersion ||
		count != len(manifest.Fields) {
		return Manifest{}, nil, ErrConflict
	}
	rows, err := tx.Query(ctx, `
SELECT field_id::text, content_hash
FROM applications.workflow_evidence_members
WHERE app_id=$1::uuid AND evidence_hash=$2
ORDER BY field_id
`, appID, hash[:])
	if err != nil {
		return Manifest{}, nil, err
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var id string
		var contentHash []byte
		if err := rows.Scan(&id, &contentHash); err != nil {
			return Manifest{}, nil, err
		}
		if index >= MaxFields || index >= len(manifest.Fields) {
			return Manifest{}, nil, ErrConflict
		}
		ref := manifest.Fields[index]
		if id != ref.FieldID || !bytes.Equal(contentHash, ref.Hash[:]) {
			return Manifest{}, nil, ErrConflict
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return Manifest{}, nil, err
	}
	if index != len(manifest.Fields) {
		return Manifest{}, nil, ErrConflict
	}
	return manifest, bytes.Clone(body), nil
}

func loadSelectedFields(ctx context.Context, tx pgx.Tx, manifest Manifest, hash [32]byte, ids []string) ([]Field, []Blob, error) {
	refs := make(map[string][32]byte, len(manifest.Fields))
	for _, ref := range manifest.Fields {
		refs[ref.FieldID] = ref.Hash
	}
	for _, id := range ids {
		if _, ok := refs[id]; !ok {
			return nil, nil, ErrInvalid
		}
	}
	if len(ids) == 0 {
		return []Field{}, []Blob{}, nil
	}
	rows, err := tx.Query(ctx, `
SELECT b.field_id::text, b.content_hash, b.body
FROM applications.workflow_evidence_members AS m
JOIN applications.workflow_evidence_blobs AS b
 ON b.app_id=m.app_id AND b.field_id=m.field_id AND b.content_hash=m.content_hash
WHERE m.app_id=$1::uuid AND m.evidence_hash=$2 AND m.field_id=ANY($3::uuid[])
ORDER BY b.field_id
`, manifest.Header.AppID, hash[:], ids)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	fields := make([]Field, 0, len(ids))
	blobs := make([]Blob, 0, len(ids))
	remaining := MaxBundleBytes
	for rows.Next() {
		var id string
		var contentHash, body []byte
		if err := rows.Scan(&id, &contentHash, &body); err != nil {
			return nil, nil, err
		}
		index := len(fields)
		if index >= len(ids) || id != ids[index] || len(body) > remaining {
			return nil, nil, ErrConflict
		}
		remaining -= len(body)
		expected := refs[id]
		if !bytes.Equal(contentHash, expected[:]) || sha256.Sum256(body) != expected {
			return nil, nil, ErrConflict
		}
		field, err := DecodeField(body)
		if err != nil || field.Definition.ID != id {
			return nil, nil, ErrConflict
		}
		fields = append(fields, field)
		blobs = append(blobs, Blob{FieldID: id, Hash: expected, Body: bytes.Clone(body)})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(fields) != len(ids) {
		return nil, nil, ErrConflict
	}
	return fields, blobs, nil
}

func (Store) ManifestInTx(ctx context.Context, tx pgx.Tx, appID string, hash [32]byte) (Manifest, error) {
	if err := storePort(ctx, tx); err != nil {
		return Manifest{}, err
	}
	if err := storeScope(appID, hash); err != nil {
		return Manifest{}, err
	}
	manifest, _, err := loadManifest(ctx, tx, appID, hash)
	if err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Empty IDs returns no fields, never all fields. The caller supplies the
// independently authorized subset; a manifest is not an authorization grant.
func (Store) FieldsInTx(ctx context.Context, tx pgx.Tx, appID string, hash [32]byte, fieldIDs []string) ([]Field, error) {
	if err := storePort(ctx, tx); err != nil {
		return nil, err
	}
	if err := storeScope(appID, hash); err != nil {
		return nil, err
	}
	ids, err := selectedIDs(fieldIDs)
	if err != nil {
		return nil, err
	}
	manifest, _, err := loadManifest(ctx, tx, appID, hash)
	if err != nil {
		return nil, err
	}
	fields, _, err := loadSelectedFields(ctx, tx, manifest, hash, ids)
	if err != nil {
		return nil, err
	}
	return fields, nil
}

func verifyReplay(ctx context.Context, tx pgx.Tx, manifest Manifest, body []byte, incoming Bundle) error {
	if !bytes.Equal(body, incoming.Manifest) {
		return ErrConflict
	}
	ids := make([]string, len(manifest.Fields))
	for i, ref := range manifest.Fields {
		ids[i] = ref.FieldID
	}
	_, blobs, err := loadSelectedFields(ctx, tx, manifest, incoming.Hash, ids)
	if err != nil {
		return err
	}
	if len(blobs) != len(incoming.Fields) {
		return ErrConflict
	}
	for i, blob := range blobs {
		expected := incoming.Fields[i]
		if blob.FieldID != expected.FieldID || blob.Hash != expected.Hash ||
			!bytes.Equal(blob.Body, expected.Body) {
			return ErrConflict
		}
	}
	return nil
}

func insertAndVerifyBlobs(ctx context.Context, tx pgx.Tx, appID string, blobs []Blob) error {
	if len(blobs) == 0 {
		return nil
	}
	ids := make([]string, len(blobs))
	hashes := make([][]byte, len(blobs))
	bodies := make([][]byte, len(blobs))
	for i := range blobs {
		ids[i] = blobs[i].FieldID
		hashes[i] = blobs[i].Hash[:]
		bodies[i] = blobs[i].Body
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO applications.workflow_evidence_blobs(app_id,field_id,content_hash,body)
SELECT $1::uuid, v.field_id, v.content_hash, v.body
FROM unnest($2::uuid[],$3::bytea[],$4::bytea[]) AS v(field_id,content_hash,body)
ORDER BY v.field_id,v.content_hash
ON CONFLICT(app_id,field_id,content_hash) DO NOTHING
`, appID, ids, hashes, bodies); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
SELECT b.field_id::text,b.content_hash,b.body
FROM unnest($2::uuid[],$3::bytea[]) AS v(field_id,content_hash)
JOIN applications.workflow_evidence_blobs AS b
 ON b.app_id=$1::uuid AND b.field_id=v.field_id AND b.content_hash=v.content_hash
ORDER BY b.field_id,b.content_hash
`, appID, ids, hashes)
	if err != nil {
		return err
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var id string
		var hash, body []byte
		if err := rows.Scan(&id, &hash, &body); err != nil {
			return err
		}
		if index >= len(blobs) {
			return ErrConflict
		}
		expected := blobs[index]
		if id != expected.FieldID || !bytes.Equal(hash, expected.Hash[:]) ||
			!bytes.Equal(body, expected.Body) {
			return ErrConflict
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if index != len(blobs) {
		return ErrConflict
	}
	return nil
}

func insertDocument(ctx context.Context, tx pgx.Tx, manifest Manifest, bundle Bundle) (bool, error) {
	header := manifest.Header
	var inserted []byte
	err := tx.QueryRow(ctx, `
INSERT INTO applications.workflow_evidence_documents(
 app_id,evidence_hash,table_id,view_id,record_id,created_by,
 schema_version,record_version,field_count,body)
VALUES($1::uuid,$2,$3::uuid,$4::uuid,$5::uuid,$6::uuid,$7,$8,$9,$10)
ON CONFLICT(app_id,evidence_hash) DO NOTHING
RETURNING evidence_hash
`, header.AppID, bundle.Hash[:], header.TableID, header.ViewID, header.RecordID,
		header.CreatedBy, header.SchemaVersion, header.RecordVersion, len(manifest.Fields), bundle.Manifest).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !bytes.Equal(inserted, bundle.Hash[:]) {
		return false, ErrConflict
	}
	return true, nil
}

func insertMembers(ctx context.Context, tx pgx.Tx, manifest Manifest, hash [32]byte) error {
	if len(manifest.Fields) == 0 {
		return nil
	}
	ids := make([]string, len(manifest.Fields))
	hashes := make([][]byte, len(manifest.Fields))
	for i := range manifest.Fields {
		ids[i] = manifest.Fields[i].FieldID
		hashes[i] = manifest.Fields[i].Hash[:]
	}
	_, err := tx.Exec(ctx, `
INSERT INTO applications.workflow_evidence_members(app_id,evidence_hash,field_id,content_hash)
SELECT $1::uuid,$2,v.field_id,v.content_hash
FROM unnest($3::uuid[],$4::bytea[]) AS v(field_id,content_hash)
ORDER BY v.field_id
`, manifest.Header.AppID, hash[:], ids, hashes)
	return err
}

func (Store) PutInTx(ctx context.Context, tx pgx.Tx, input Bundle) (created bool, err error) {
	if err = storePort(ctx, tx); err != nil {
		return false, err
	}
	manifest, bundle, err := incomingBundle(input)
	if err != nil {
		return false, err
	}
	child, err := tx.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		rollbackErr := child.Rollback(rollbackCtx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
		if err != nil {
			created = false
		}
	}()
	prior, body, lookupErr := loadManifest(ctx, child, manifest.Header.AppID, bundle.Hash)
	if lookupErr == nil {
		if err = verifyReplay(ctx, child, prior, body, bundle); err != nil {
			return false, err
		}
		if err = child.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	if !errors.Is(lookupErr, ErrMissing) {
		return false, lookupErr
	}
	if err = insertAndVerifyBlobs(ctx, child, manifest.Header.AppID, bundle.Fields); err != nil {
		return false, err
	}
	inserted, err := insertDocument(ctx, child, manifest, bundle)
	if err != nil {
		return false, err
	}
	if inserted {
		if err = insertMembers(ctx, child, manifest, bundle.Hash); err != nil {
			return false, err
		}
	} else {
		prior, body, err = loadManifest(ctx, child, manifest.Header.AppID, bundle.Hash)
		if err != nil {
			return false, err
		}
		if err = verifyReplay(ctx, child, prior, body, bundle); err != nil {
			return false, err
		}
	}
	if err = child.Commit(ctx); err != nil {
		return false, err
	}
	return inserted, nil
}
