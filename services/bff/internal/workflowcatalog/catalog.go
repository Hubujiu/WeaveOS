package workflowcatalog

import (
 "context"
 "errors"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
 "github.com/jackc/pgx/v5"
)

var ErrInvalid=errors.New("invalid workflow catalog request")
var ErrMissing=errors.New("workflow catalog resource missing")
var ErrConflict=errors.New("workflow catalog version conflict")
var ErrNotReady=errors.New("workflow catalog not ready")
var ErrClosing=errors.New("workflow catalog closing")
type Catalog struct{}
type VersionInput struct {
 AppID,TableID,ViewID,FlowID,ActorID,Name string
 ExpectedRevision,ExpectedSchemaVersion int64
 Graph flowgraph.Graph
 AllowWithdraw bool
}
type Head struct {
 FlowID,AppID,TableID,ViewID,Name,State string
 Revision,CurrentVersion,CandidateVersion int64
}
type ReserveInput struct {
 AppID,FlowID,InstanceID,RecordID,ActorID string
 ExpectedRevision,ExpectedSchemaVersion,ExpectedRecordVersion int64
}
type Instance struct {
 ID,FlowID,AppID,TableID,ViewID,RecordID,InitiatorID,State string
 DefinitionVersion,Sequence int64
}
type Conflict struct {
 FlowID,NodeID,FieldID,Reason string
 Version int64
}
func(Catalog) PutVersionInTx(context.Context,pgx.Tx,VersionInput)(Head,error){return Head{},ErrNotReady}
func(Catalog) ConfirmDeploymentInTx(context.Context,pgx.Tx,string,string,int64,string)(Head,error){return Head{},ErrNotReady}
func(Catalog) EnableInTx(context.Context,pgx.Tx,string,string,int64)(Head,error){return Head{},ErrNotReady}
func(Catalog) RequestCloseInTx(context.Context,pgx.Tx,string,string,int64)(Head,error){return Head{},ErrNotReady}
func(Catalog) FinalizeCloseInTx(context.Context,pgx.Tx,string,string,int64)(Head,error){return Head{},ErrNotReady}
func(Catalog) GetInTx(context.Context,pgx.Tx,string,string)(Head,error){return Head{},ErrNotReady}
func(Catalog) ReserveInTx(context.Context,pgx.Tx,ReserveInput)(Instance,error){return Instance{},ErrNotReady}
func(Catalog) CheckCompatibilityInTx(context.Context,pgx.Tx,string,string,[]appquery.Field)([]Conflict,error){return nil,ErrNotReady}
