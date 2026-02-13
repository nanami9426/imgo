package utils

type StatCode int

const (
	StatSuccess         StatCode = 0
	StatInvalidParam    StatCode = 1001
	StatUnauthorized    StatCode = 1002
	StatForbidden       StatCode = 1003
	StatNotFound        StatCode = 1004
	StatConflict        StatCode = 1005
	StatTooManyRequests StatCode = 1006
	StatInternalError   StatCode = 2000
	StatDatabaseError   StatCode = 2001
)

var statText = map[StatCode]string{
	StatSuccess:         "success",
	StatInvalidParam:    "invalid params",
	StatUnauthorized:    "unauthorized",
	StatForbidden:       "forbidden",
	StatNotFound:        "not found",
	StatConflict:        "conflict",
	StatTooManyRequests: "too many requests",
	StatInternalError:   "internal error",
	StatDatabaseError:   "database error",
}

func StatText(code StatCode) string {
	if msg, ok := statText[code]; ok {
		return msg
	}
	return "unknown error"
}
