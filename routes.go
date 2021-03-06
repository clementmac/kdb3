package main

import (
	"expvar"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"

	"github.com/gorilla/mux"
)

type Route struct {
	Name        string
	Methods     string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.Handle("/_debug/vars", expvar.Handler())
	router.HandleFunc("/_debug/pprof", pprof.Index)
	router.Handle("/_debug/allocs", pprof.Handler("allocs"))
	router.Handle("/_debug/block", pprof.Handler("block"))
	router.Handle("/_debug/cmdline", pprof.Handler("cmdline"))
	router.Handle("/_debug/goroutine", pprof.Handler("goroutine"))
	router.Handle("/_debug/heap", pprof.Handler("heap"))
	router.Handle("/_debug/mutex", pprof.Handler("mutex"))
	router.Handle("/_debug/profile", pprof.Handler("profile"))
	router.Handle("/_debug/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/_debug/trace", pprof.Handler("trace"))

	router.PathPrefix("/_utils").
		Handler(http.StripPrefix("/_utils", http.FileServer(http.Dir("./share/www/"))))

	for _, route := range routes {
		router.
			Methods(route.Methods).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}

	return router
}

var routes = Routes{
	Route{
		"Info",
		"GET",
		"/",
		GetInfo,
	},
	Route{
		"AllDatabases",
		"GET",
		"/_all_dbs",
		AllDatabases,
	},
	Route{
		"UUID",
		"GET",
		"/_uuids",
		GetUUIDs,
	},
	Route{
		"GetDatabase",
		"GET",
		"/{db}",
		GetDatabase,
	},
	Route{
		"PutDatabase",
		"PUT",
		"/{db}",
		PutDatabase,
	},
	Route{
		"PostDatabase",
		"POST",
		"/{db}",
		PutDocument,
	},
	Route{
		"DeleteDatabase",
		"DELETE",
		"/{db}",
		DeleteDatabase,
	},
	Route{
		"DatabaseAllDocs",
		"GET",
		"/{db}/_all_docs",
		DatabaseAllDocs,
	},
	Route{
		"BulkPutDocuments",
		"POST",
		"/{db}/_bulk_docs",
		BulkPutDocuments,
	},
	Route{
		"BulkGetDocuments",
		"POST",
		"/{db}/_bulk_gets",
		BulkGetDocuments,
	},
	Route{
		"DatabaseChanges",
		"GET",
		"/{db}/_changes",
		DatabaseChanges,
	},
	Route{
		"DatabaseCompact",
		"POST",
		"/{db}/_compact",
		DatabaseCompact,
	},
	Route{
		"GetDocument",
		"GET",
		"/{db}/{docid}",
		GetDocument,
	},
	Route{
		"HeadDocument",
		"HEAD",
		"/{db}/{docid}",
		HeadDocument,
	},
	Route{
		"PutDocument",
		"PUT",
		"/{db}/{docid}",
		PutDocument,
	},
	Route{
		"DeleteDocument",
		"DELETE",
		"/{db}/{docid}",
		DeleteDocument,
	},
	Route{
		"GetDDocument",
		"GET",
		"/{db}/_design/{docid}",
		GetDDocument,
	},
	Route{
		"PutDDocument",
		"PUT",
		"/{db}/_design/{docid}",
		PutDDocument,
	},
	Route{
		"DeleteDDocument",
		"DELETE",
		"/{db}/_design/{docid}",
		DeleteDDocument,
	},
	Route{
		"SelectView",
		"GET",
		"/{db}/_design/{docid}/{view}",
		SelectView,
	},
	Route{
		"SelectView",
		"GET",
		"/{db}/_design/{docid}/{view}/{select}",
		SelectView,
	},
}
