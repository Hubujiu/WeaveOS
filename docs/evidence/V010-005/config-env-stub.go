package main

import (
 "context"
 "net/http"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
)

type config struct { DatabaseURL,RedisURL,Origin,Generation,AuditKeyID string;AuditKey []byte }
func readConfig(func(string)string)(config,error){return config{},nil}
func buildHandler(context.Context,config) (http.Handler,func(),error) {return httpserver.NewHandler(nil),func(){},nil}
