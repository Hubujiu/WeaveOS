package org.weaveos.workflow;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.CountDownLatch;
/** Dedicated synthetic transport fixture. Not a production bootstrap. */
public final class RootExecutionRpcFixtureMain {
 public static void main(String[] args)throws Exception{
  var f=new RootExecutionRegistryTest();f.setup();
  var server=io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder.forPort(0)
   .maxInboundMessageSize(1024*1024+16384)
   .addService(new DeploymentGrpcService(f.deployments))
   .addService(new ExecutionGrpcService(f.registry)).build().start();
  Runtime.getRuntime().addShutdownHook(new Thread(()->{server.shutdownNow();try{server.awaitTermination(10,java.util.concurrent.TimeUnit.SECONDS);}catch(InterruptedException e){Thread.currentThread().interrupt();}f.cleanup();}));
  Files.writeString(Path.of("/tmp/weaveos-rpc-ready"),Integer.toString(server.getPort()));
  new CountDownLatch(1).await();
 }
}
