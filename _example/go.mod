module github.com/go-chi/httplog/v3/_example

go 1.22.6

replace github.com/go-chi/httplog/v3 => ../

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/httplog/v3 v3.0.0-00010101000000-000000000000
	github.com/go-chi/traceid v0.3.0
	github.com/golang-cz/devslog v0.0.9
)

require github.com/google/uuid v1.6.0 // indirect
