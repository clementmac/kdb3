package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Database struct {
	name            string
	updateSeqNumber int
	updateSeqID     string

	dbPath  string
	readers DatabaseReaderPool
	writer  DatabaseWriter
	mux     sync.Mutex

	changeSeq *ChangeSequenceGenarator
	idSeq     *SequenceUUIDGenarator
	viewmgr   ViewManager
}

func NewDatabase(name, dbPath, viewPath string, createIfNotExists bool) (*Database, error) {
	path := filepath.Join(dbPath, name+dbExt)
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if !createIfNotExists {
			return nil, errors.New("db_not_found")
		}
	} else {
		if createIfNotExists {
			return nil, errors.New("db_exists")
		}
	}

	db := &Database{name: name, dbPath: path}
	db.idSeq = NewSequenceUUIDGenarator()

	connStr := db.dbPath + "?_journal=WAL"
	db.writer = new(DefaultDatabaseWriter)
	db.writer.Open(connStr)
	db.readers = NewDatabaseReaderPool(connStr, 4)

	if createIfNotExists {
		db.writer.Begin()
		if err := db.writer.ExecBuildScript(); err != nil {
			return nil, err
		}
		db.writer.Commit()
	}

	db.Open()
	db.viewmgr = NewViewManager(path, viewPath, name)

	if createIfNotExists {
		err = db.viewmgr.SetupViews(db)
		if err != nil {
			return nil, err
		}
	}

	err = db.viewmgr.Initialize(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *Database) Open() error {
	db.updateSeqNumber, db.updateSeqID = db.GetLastUpdateSequence()
	db.changeSeq = NewChangeSequenceGenarator(138, db.updateSeqNumber, db.updateSeqID)
	return nil
}

func (db *Database) Close() error {
	db.viewmgr.Close()
	db.writer.Close()
	db.readers.Close()
	return nil
}

func (db *Database) PutDocument(newDoc *Document) (*Document, error) {

	writer := db.writer

	db.mux.Lock()
	defer db.mux.Unlock()

	err := writer.Begin()
	defer writer.Rollback()
	if err != nil {
		return nil, err
	}

	if newDoc.ID == "" {
		newDoc.ID = db.idSeq.Next()
	}

	currentDoc, err := writer.GetDocumentRevisionByID(newDoc.ID)
	if err != nil && err.Error() != "doc_not_found" {
		return nil, err
	}

	if currentDoc != nil && !currentDoc.Deleted && currentDoc.Version != newDoc.Version {
		return nil, errors.New("doc_conflict")
	}

	if currentDoc != nil && currentDoc.Deleted {
		newDoc.Version = currentDoc.Version
	}

	newDoc.CalculateVersion()

	updateSeqNumber, updateSeqID := db.changeSeq.Next()

	err = writer.PutDocument(updateSeqNumber, updateSeqID, newDoc, currentDoc)
	if err != nil {
		return nil, err
	}

	if err := writer.Commit(); err != nil {
		return nil, err
	}

	db.updateSeqNumber = updateSeqNumber
	db.updateSeqID = updateSeqID

	if strings.HasPrefix(newDoc.ID, "_design/") {
		db.viewmgr.UpdateDesignDocument(newDoc.ID, newDoc.Data)
	}

	doc := Document{
		ID:      newDoc.ID,
		Version: newDoc.Version,
		Deleted: newDoc.Deleted,
	}

	return &doc, nil
}

func (db *Database) GetDocument(doc *Document, includeData bool) (*Document, error) {

	reader := db.readers.Borrow()
	defer db.readers.Return(reader)

	reader.Begin()
	defer reader.Commit()

	if includeData {
		if doc.Version > 0 {
			return reader.GetDocumentByIDandVersion(doc.ID, doc.Version)
		}
		return reader.GetDocumentByID(doc.ID)

	}
	if doc.Version > 0 {
		return reader.GetDocumentRevisionByIDandVersion(doc.ID, doc.Version)
	}
	return reader.GetDocumentRevisionByID(doc.ID)
}

func (db *Database) GetAllDesignDocuments() ([]*Document, error) {
	reader := db.readers.Borrow()
	defer db.readers.Return(reader)

	reader.Begin()
	defer reader.Commit()

	return reader.GetAllDesignDocuments()
}

func (db *Database) DeleteDocument(doc *Document) (*Document, error) {
	doc.Deleted = true
	return db.PutDocument(doc)
}

func (db *Database) GetLastUpdateSequence() (int, string) {
	reader := db.readers.Borrow()
	defer db.readers.Return(reader)

	reader.Begin()
	defer reader.Commit()

	return reader.GetLastUpdateSequence()
}

func (db *Database) GetChanges() []byte {
	reader := db.readers.Borrow()
	defer db.readers.Return(reader)

	reader.Begin()
	defer reader.Commit()

	return reader.GetChanges()
}

func (db *Database) GetDocumentCount() int {
	reader := db.readers.Borrow()
	defer db.readers.Return(reader)

	reader.Begin()
	defer reader.Commit()

	return reader.GetDocumentCount()
}

func (db *Database) GetSQLiteVersion() string {
	reader := db.readers.Borrow()
	defer db.readers.Return(reader)

	reader.Begin()
	defer reader.Commit()

	return reader.GetSQLiteVersion()
}

func (db *Database) Stat() *DBStat {
	stat := &DBStat{}
	stat.DBName = db.name
	stat.UpdateSeq = formatSeq(db.updateSeqNumber, db.updateSeqID)
	stat.DocCount = db.GetDocumentCount()
	return stat
}

func (db *Database) Vacuum() error {
	db.viewmgr.Vacuum()
	return db.writer.Vacuum()
}

func (db *Database) SelectView(ddocID, viewName, selectName string, values url.Values, stale bool) ([]byte, error) {
	return db.viewmgr.SelectView(db.updateSeqNumber, db.updateSeqID, ddocID, viewName, selectName, values, stale)
}
