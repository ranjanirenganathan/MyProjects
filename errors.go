package models



type QueryError struct {
	message string
}

type NotFoundError struct {
	*QueryError
}

type DuplicateError struct {
	*QueryError
}

type ValidationError struct {
	*QueryError
	Errors []error
}

type InvalidIdError struct {
	*QueryError
}

func (e *QueryError) Error() string {
	return e.message
}
