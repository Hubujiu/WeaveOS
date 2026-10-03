package appquery

const MaxJSONVersion int64 = 9007199254740991

// PageWindow validates arbitrary record pages before binding LIMIT/OFFSET.
// A page after the current total is valid and yields an empty result.
func PageWindow(page, pageSize int64) (limit, offset int64, err error) {
	if page < 1 || page > MaxJSONVersion || pageSize < 1 || pageSize > 100 || page-1 > MaxJSONVersion/pageSize {
		return 0, 0, ErrInvalid
	}
	return pageSize, (page - 1) * pageSize, nil
}
