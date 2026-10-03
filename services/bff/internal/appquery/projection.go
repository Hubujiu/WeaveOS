package appquery

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrProjectionInvalid = errors.New("invalid authorized projection stream")

type Projection struct {
	Total         int64
	Fingerprint   string
	StreamedBytes int64 // canonical row JSONB bytes transferred through the Go stream
}

// FingerprintRows consumes one ordered-column JSONB row per result from an
// authorized, deterministic SQL cursor. The caller constructs that SELECT from
// all currently readable fields and reference display in the same RR snapshot.
func FingerprintRows(ctx context.Context, rows pgx.Rows) (Projection, error) {
	if rows == nil {
		return Projection{}, ErrProjectionInvalid
	}
	defer rows.Close()
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos/apprecords/projection/v1\x00"))
	var result Projection
	var length [8]byte
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return Projection{}, err
		}
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return Projection{}, err
		}
		if len(raw) == 0 || result.Total >= 9007199254740991 {
			return Projection{}, ErrProjectionInvalid
		}
		if int64(len(raw)) > 9007199254740991-result.StreamedBytes {
			return Projection{}, ErrProjectionInvalid
		}
		result.StreamedBytes += int64(len(raw))
		binary.BigEndian.PutUint64(length[:], uint64(len(raw)))
		_, _ = h.Write(length[:])
		_, _ = h.Write(raw)
		result.Total++
	}
	if err := rows.Err(); err != nil {
		return Projection{}, err
	}
	binary.BigEndian.PutUint64(length[:], uint64(result.Total))
	_, _ = h.Write(length[:])
	result.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return result, nil
}
