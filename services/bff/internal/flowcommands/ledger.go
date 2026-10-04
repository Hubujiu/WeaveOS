package flowcommands

import (
 "context"
 "errors"
 "github.com/jackc/pgx/v5"
)
var ErrMissing=errors.New("workflow command not found")
type Entry struct { Command Command; State string; Receipt *Receipt }
type Ledger struct { Namespace string }
func (Ledger) AcceptInTx(context.Context,pgx.Tx,Command)(Entry,error){return Entry{},errNotImplementedLedger}
func (Ledger) GetInTx(context.Context,pgx.Tx,string)(Entry,error){return Entry{},errNotImplementedLedger}
func (Ledger) ApplyInTx(context.Context,pgx.Tx,Command,Receipt,int64,func(context.Context,pgx.Tx,ApplyPlan)error)(Entry,error){return Entry{},errNotImplementedLedger}
var errNotImplementedLedger=errors.New("root RED-only ledger declaration")
