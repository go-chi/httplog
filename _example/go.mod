module github.com/golang-cz/httplog/_example

go 1.22.6

replace github.com/golang-cz/httplog => ../

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/traceid v0.2.0
	github.com/golang-cz/devslog v0.0.9
	github.com/golang-cz/httplog v0.0.0-20240820102836-395a380e1ae5
)

require github.com/google/uuid v1.6.0 // indirect
