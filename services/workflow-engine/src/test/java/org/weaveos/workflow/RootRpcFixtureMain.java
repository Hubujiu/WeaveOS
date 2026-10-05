package org.weaveos.workflow;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.CountDownLatch;
/** Root-owned synthetic fixture, never a production service bootstrap. */
public final class RootRpcFixtureMain {
 public static void main(String[] args)throws Exception{
  RootDeploymentGrpcIT fixture=new RootDeploymentGrpcIT();
  fixture.setup();
  Runtime.getRuntime().addShutdownHook(new Thread(fixture::cleanup));
  Files.writeString(Path.of("/tmp/weaveos-rpc-ready"),Integer.toString(fixture.server.getPort()));
  new CountDownLatch(1).await();
 }
}
