package schema

type Column struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Default  string `json:"default,omitempty"`
}

type ForeignKey struct {
	Columns    []string            `json:"columns"`
	References map[string][]string `json:"references"`
}

type Constraints struct {
	Primary     []string     `json:"primary"`
	Unique      [][]string   `json:"unique"`
	ForeignKeys []ForeignKey `json:"foreign_keys"`
}

type Table struct {
	Name        string            `json:"name"`
	Columns     map[string]Column `json:"columns"`
	Constraints Constraints       `json:"constraints"`
	Indexes     [][]string        `json:"indexes"`
}

type Node struct {
	Name     string `json:"name"`
	User     string `json:"user"`
	Database string `json:"db"`
	Password string `json:"password"`
}
