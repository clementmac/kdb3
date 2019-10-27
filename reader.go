package main

import (
	"database/sql"
	"errors"
	"fmt"
)

type DatabaseReader interface {
	Open(path string) error
	Close() error

	GetDocumentRevisionByIDandVersion(ID string, Version int) (*Document, error)
	GetDocumentRevisionByID(ID string) (*Document, error)

	GetDocumentByID(ID string) (*Document, error)
	GetDocumentByIDandVersion(ID string, Version int) (*Document, error)

	GetAllDesignDocuments() ([]*Document, error)
	GetChanges() []byte

	GetLastUpdateSequence() (int, string)
	GetDocumentCount() int
	GetSQLiteVersion() string
}

type DefaultDatabaseReader struct {
	conn *sql.DB
}

func (reader *DefaultDatabaseReader) Open(path string) error {
	con, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	reader.conn = con
	return nil
}

func (reader *DefaultDatabaseReader) GetDocumentRevisionByIDandVersion(ID string, Version int) (*Document, error) {
	doc := Document{}

	row := reader.conn.QueryRow("SELECT doc_id, version, deleted FROM changes WHERE doc_id = ? AND version = ? LIMIT 1", ID, Version)
	err := row.Scan(&doc.ID, &doc.Version, &doc.Deleted)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, err
	}

	if doc.ID == "" {
		return nil, errors.New("doc_not_found")
	}

	if doc.Deleted == true {
		return &doc, errors.New("doc_not_found")
	}

	return &doc, nil
}

func (reader *DefaultDatabaseReader) GetDocumentRevisionByID(ID string) (*Document, error) {
	doc := Document{}

	row := reader.conn.QueryRow("SELECT doc_id, version, deleted FROM changes WHERE doc_id = ? ORDER BY version DESC LIMIT 1", ID)
	err := row.Scan(&doc.ID, &doc.Version, &doc.Deleted)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, err
	}

	if doc.ID == "" {
		return nil, errors.New("doc_not_found")
	}

	if doc.Deleted == true {
		return &doc, errors.New("doc_not_found")
	}

	return &doc, nil
}

func (reader *DefaultDatabaseReader) GetDocumentByID(ID string) (*Document, error) {
	doc := &Document{}

	row := reader.conn.QueryRow("SELECT doc_id, version, deleted, (SELECT data FROM documents WHERE doc_id = ?) FROM changes WHERE doc_id = ? ORDER BY version DESC LIMIT 1", ID, ID)
	err := row.Scan(&doc.ID, &doc.Version, &doc.Deleted, &doc.Data)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, err
	}

	if doc.ID == "" {
		return nil, errors.New("doc_not_found")
	}

	if doc.Deleted == true {
		return doc, errors.New("doc_not_found")
	}

	return doc, nil
}

func (reader *DefaultDatabaseReader) GetDocumentByIDandVersion(ID string, Version int) (*Document, error) {
	doc := &Document{}

	row := reader.conn.QueryRow("SELECT doc_id, version, deleted, (SELECT data FROM documents WHERE doc_id = ?) as data FROM changes WHERE doc_id = ? AND version = ? LIMIT 1", ID, ID, Version)
	err := row.Scan(&doc.ID, &doc.Version, &doc.Deleted, &doc.Data)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, err
	}

	if doc.ID == "" {
		return nil, errors.New("doc_not_found")
	}

	if doc.Deleted == true {
		return doc, errors.New("doc_not_found")
	}

	return doc, nil
}

func (reader *DefaultDatabaseReader) GetAllDesignDocuments() ([]*Document, error) {

	var docs []*Document
	rows, err := reader.conn.Query("SELECT doc_id FROM documents WHERE doc_id like '_design/%'")
	if err != nil {
		return nil, err
	}

	for {
		if !rows.Next() {
			break
		}

		var id string
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}

		doc, _ := reader.GetDocumentByID(id)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (db *DefaultDatabaseReader) GetChanges() []byte {
	sqlGetChanges := `WITH all_changes(seq, version, doc_id, deleted) as
	(
		SELECT printf('%d-%s', seq_number, seq_id) as seq, version, doc_id, deleted FROM changes c ORDER by seq_id DESC
	),
	changes_object (obj) as
	(
		SELECT (CASE WHEN deleted != 1 THEN JSON_OBJECT('seq', seq, 'version', version, 'id', doc_id) ELSE JSON_OBJECT('seq', seq, 'version', version, 'id', doc_id, 'deleted', true)  END) as obj FROM all_changes
	)
	SELECT JSON_OBJECT('results',JSON_GROUP_ARRAY(obj)) FROM changes_object`
	row := db.conn.QueryRow(sqlGetChanges)
	var (
		changes []byte
	)

	err := row.Scan(&changes)
	if err != nil {
		panic(err)
	}

	return changes
}

func (db *DefaultDatabaseReader) GetLastUpdateSequence() (int, string) {
	sqlGetMaxSeq := "SELECT seq_number, seq_id FROM (SELECT MAX(seq_number) as seq_number, MAX(seq_id)  as seq_id FROM changes WHERE seq_id = (SELECT MAX(seq_id) FROM changes) UNION ALL SELECT 0, '') WHERE seq_number IS NOT NULL LIMIT 1"
	row := db.conn.QueryRow(sqlGetMaxSeq)
	var (
		maxSeqNumber int
		maxSeqID     string
	)

	err := row.Scan(&maxSeqNumber, &maxSeqID)
	if err != nil {
		panic(err)
	}

	return maxSeqNumber, maxSeqID
}

func (db *DefaultDatabaseReader) GetDocumentCount() int {
	row := db.conn.QueryRow("SELECT COUNT(1) FROM documents")
	count := 0
	row.Scan(&count)
	return count
}

func (db *DefaultDatabaseReader) GetSQLiteVersion() string {
	row := db.conn.QueryRow("SELECT sqlite_version() as version")
	version := ""
	row.Scan(&version)
	return version
}

func (reader *DefaultDatabaseReader) Close() error {
	return reader.conn.Close()
}

type DatabaseReaderPool struct {
	path string
	pool chan DatabaseReader
}

func NewDatabaseReaderPool(path string, max int) *DatabaseReaderPool {
	return &DatabaseReaderPool{
		path: path,
		pool: make(chan DatabaseReader, max),
	}
}

func (p *DatabaseReaderPool) Borrow() DatabaseReader {
	var r DatabaseReader
	select {
	case r = <-p.pool:
	default:
		r = &DefaultDatabaseReader{}
		err := r.Open(p.path)
		if err != nil {
			fmt.Println(err)
		}
	}

	return r
}

func (p *DatabaseReaderPool) Return(r DatabaseReader) {
	select {
	case p.pool <- r:
	default:
	}
}

func (p *DatabaseReaderPool) Close() {
	for {
		var r DatabaseReader
		select {
		case r = <-p.pool:
			r.Close()
		default:
		}
		if r == nil {
			break
		}
	}
}
