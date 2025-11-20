package lancedb

// DeleteBuilder provides a fluent interface for deleting rows from a table.
// It offers a consistent API with the Query builder pattern.
type DeleteBuilder struct {
	table     *Table
	predicate string
}

// DeleteBuilder creates a new DeleteBuilder for the table.
// Use this method for builder-pattern style deletions.
//
// Example:
//
//	err := table.DeleteBuilder().Where("id > 100").Execute()
func (t *Table) DeleteBuilder() *DeleteBuilder {
	return &DeleteBuilder{
		table: t,
	}
}

// Where sets the predicate for the delete operation.
// The predicate is a SQL-like expression (e.g., "id > 100" or "category = 'outdated'").
//
// Example:
//
//	err := table.Delete().Where("name = 'old document'").Execute()
func (d *DeleteBuilder) Where(predicate string) *DeleteBuilder {
	d.predicate = predicate
	return d
}

// Execute performs the delete operation with the configured predicate.
// This method automatically compacts the table to reclaim disk space.
// Returns an error if the predicate is empty or the delete operation fails.
func (d *DeleteBuilder) Execute() error {
	if d.predicate == "" {
		return &Error{Message: "predicate must be set using Where() before calling Execute()"}
	}

	// Use the simple Delete method which handles the C API call
	return d.table.Delete(d.predicate)
}

