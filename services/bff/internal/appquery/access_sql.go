package appquery

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/jackc/pgx/v5"
)

var ErrForbidden = errors.New("record query field or row forbidden")

// AccessSQL is a trusted, permission-conditioned SQL fragment for one form.
// Projection omits fields that this row cannot read; reference display still
// requires the authoritative registry resolver in the same RR snapshot.
type AccessSQL struct {
	RowPredicate, Projection string
	Arguments                []any
}
type SearchSQL struct {
	Access         AccessSQL
	Filter         Plan // page/full projection binds, including sort
	Arguments      []any
	CountFilter    Plan // compact binds; excludes actor used only by field projection
	CountArguments []any
}

// CompileSearch composes permission-conditioned rows/fields with typed
// predicates only after filter/sort field scopes cover all visible rows.
func CompileSearch(policy appaccess.Policy, fields []Field, filterRaw, sortRaw json.RawMessage, firstParameter int) (SearchSQL, error) {
	access, err := CompileAccess(policy, fields, firstParameter)
	if err != nil {
		return SearchSQL{}, err
	}
	filter, err := Compile(filterRaw, sortRaw, fields, firstParameter+len(access.Arguments))
	if err != nil {
		return SearchSQL{}, err
	}
	if !policy.CoversRead(filter.ReferencedFields) {
		return SearchSQL{}, ErrForbidden
	}
	args := append(append([]any{}, access.Arguments...), filter.Arguments...)
	var countArgs []any
	if policy.VisibleScope() == appaccess.Own {
		countArgs = []any{policy.ActorID}
	}
	countFilter, err := Compile(filterRaw, nil, fields, firstParameter+len(countArgs))
	if err != nil {
		return SearchSQL{}, err
	}
	countArgs = append(countArgs, countFilter.Arguments...)
	return SearchSQL{Access: access, Filter: filter, Arguments: args, CountFilter: countFilter, CountArguments: countArgs}, nil
}

func CompileAccess(policy appaccess.Policy, fields []Field, firstParameter int) (AccessSQL, error) {
	var result AccessSQL
	if firstParameter < 1 || firstParameter > 65000 || !uuidPattern.MatchString(policy.ActorID) {
		return result, ErrInvalid
	}
	visible := policy.VisibleScope()
	if visible == appaccess.None {
		return result, ErrForbidden
	}
	copyFields := append([]Field(nil), fields...)
	sort.Slice(copyFields, func(i, j int) bool { return copyFields[i].ID < copyFields[j].ID })
	var pieces []string
	needsActor := visible == appaccess.Own
	for i, field := range copyFields {
		if !uuidPattern.MatchString(field.ID) || !validKind(field.Kind) || i > 0 && copyFields[i-1].ID == field.ID {
			return result, ErrInvalid
		}
		scope := policy.FieldScope(appaccess.Read, field.ID)
		if scope == appaccess.None {
			continue
		}
		column := "r." + pgx.Identifier{"f_" + strings.ReplaceAll(field.ID, "-", "")}.Sanitize()
		value := column
		switch field.Kind {
		case Number, Money, Date, SingleSelect, Member, Department:
			value = column + "::text"
		case Datetime:
			value = "to_char(" + column + ` AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')`
		}
		cell := fmt.Sprintf("jsonb_build_object('%s',%s)", field.ID, value)
		if scope == appaccess.Own {
			needsActor = true
			cell = "CASE WHEN r.created_by=$" + fmt.Sprint(firstParameter) + "::uuid THEN " + cell + " ELSE '{}'::jsonb END"
		}
		pieces = append(pieces, cell)
	}
	if len(pieces) == 0 {
		return result, ErrForbidden
	}
	if needsActor {
		result.Arguments = []any{policy.ActorID}
	}
	if visible == appaccess.Own {
		result.RowPredicate = "r.created_by=$" + fmt.Sprint(firstParameter) + "::uuid"
	} else {
		result.RowPredicate = "TRUE"
	}
	result.Projection = "'{}'::jsonb || " + strings.Join(pieces, " || ")
	return result, nil
}
