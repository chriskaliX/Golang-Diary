package designpattern

type Database interface {
	GetData(string) string
	PutData(string, string)
}

type mongoDB struct {
	database map[string]string
}

type sqlite struct {
	database map[string]string
}

func (m mongoDB) GetData(query string) string {
	if value, ok := m.database[query]; ok {
		return value
	}
	return ""
}

func (m mongoDB) PutData(query, data string) {
	m.database[query] = data
}

func (s sqlite) GetData(query string) string {
	if value, ok := s.database[query]; ok {
		return value
	}
	return ""
}

func (s sqlite) PutData(query, data string) {
	s.database[query] = data
}

func DatabaseFactory(env string) Database {
	switch env {
	case "prd":
		return mongoDB{
			database: make(map[string]string),
		}
	case "dev":
		return sqlite{
			database: make(map[string]string),
		}
	default:
		return nil
	}
}
