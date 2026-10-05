package appquery

import (
	"strings"

	"github.com/jackc/pgx/v5"
)

// StoredValueSQL represents actual typed values without new-write rounding or
// truncation. The caller owns row/field authorization and relation scoping.
func StoredValueSQL(field Field) (string, error) {
	if !uuidPattern.MatchString(field.ID) || !validKind(field.Kind) {
		return "", ErrInvalid
	}
	column := "r." + pgx.Identifier{"f_" + strings.ReplaceAll(field.ID, "-", "")}.Sanitize()
	switch field.Kind {
	case Number, Money, SingleSelect, Member, Department:
		return column + "::text", nil
	case Date:
		return "to_char(" + column + ",'YYYY-MM-DD')", nil
	case Datetime:
		return "to_char(" + column + ` AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')`, nil
	default:
		return column, nil
	}
}
