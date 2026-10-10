//go:build workflowruntime_integration && workflowrpc_integration && linux
package main
import("os";"strings";"testing")
func TestRootDiagnosticChildLogLivesThroughOwningCleanup(t *testing.T){
 var logBytes []byte; var readErr error
 t.Run("same_owner_cleanup_order",func(t *testing.T){
  var child *rootBFFProcess
  // Same registration order as actual rootFormalSetupEditable.
  t.Cleanup(func(){rootFormalStop(child,false); logBytes,readErr=os.ReadFile(child.output)})
  child=rootFormalStartChild(t,"diagnostic","/bin/sh",[]string{"-c","printf diagnostic-child-output"},map[string]string{})
  <-child.done
 })
 if readErr!=nil||!strings.Contains(string(logBytes),"diagnostic-child-output"){t.Fatalf("owning cleanup lost exited child's actual diagnostic: read=%v bytes=%q",readErr,logBytes)}
}
