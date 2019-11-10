package main

type DBStat struct {
	DBName    string `json:"db_name"`
	UpdateSeq string `json:"update_seq"`
	DocCount  int    `json:"doc_count"`
}

type DesignDocumentView struct {
	Setup  []string          `json:"setup,omitempty"`
	Delete []string          `json:"delete,omitempty"`
	Update []string          `json:"update,omitempty"`
	Select map[string]string `json:"select,omitempty"`
}

type DesignDocument struct {
	ID      string                         `json:"_id"`
	Version int                            `json:"_version,omitempty"`
	Views   map[string]*DesignDocumentView `json:"views"`
}

type Query struct {
	text   string
	params []string
}
