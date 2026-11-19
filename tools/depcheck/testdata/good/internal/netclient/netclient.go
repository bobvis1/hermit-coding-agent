package netclient

import "net/http"

func New() *http.Client { return &http.Client{} }
