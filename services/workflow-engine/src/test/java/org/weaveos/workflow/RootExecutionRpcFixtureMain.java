package org.weaveos.workflow;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.CountDownLatch;
/** Dedicated synthetic transport fixture. Not a production bootstrap. */
public final class RootExecutionRpcFixtureMain {
 public static void main(String[] args)throws Exception{
  var f=new RootExecutionRegistryTest();f.setup();
  var health=new io.grpc.health.v1.HealthGrpc.HealthImplBase(){
   @Override public void check(io.grpc.health.v1.HealthCheckRequest request,io.grpc.stub.StreamObserver<io.grpc.health.v1.HealthCheckResponse> response){
    try{f.jdbc.queryForObject("SELECT 1",Integer.class);response.onNext(io.grpc.health.v1.HealthCheckResponse.newBuilder().setStatus(io.grpc.health.v1.HealthCheckResponse.ServingStatus.SERVING).build());response.onCompleted();}
    catch(Exception e){response.onError(io.grpc.Status.UNAVAILABLE.asRuntimeException());}
   }
  };
  var server=io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder.forPort(0)
   .maxInboundMessageSize(1024*1024+16384)
   .addService(health)
   .addService(new DeploymentGrpcService(f.deployments))
   .addService(new ExecutionGrpcService(f.registry)).build().start();
  Runtime.getRuntime().addShutdownHook(new Thread(()->{server.shutdownNow();try{server.awaitTermination(10,java.util.concurrent.TimeUnit.SECONDS);}catch(InterruptedException e){Thread.currentThread().interrupt();}f.cleanup();}));
  Files.writeString(Path.of("/tmp/weaveos-rpc-ready"),Integer.toString(server.getPort()));
  new CountDownLatch(1).await();
 }
}
