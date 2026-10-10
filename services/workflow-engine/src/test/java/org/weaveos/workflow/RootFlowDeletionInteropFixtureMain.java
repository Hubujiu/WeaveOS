package org.weaveos.workflow;

/** Isolated actual runtime for the tagged Go client proof, not a production entrypoint. */
public final class RootFlowDeletionInteropFixtureMain {
 public static void main(String[] args)throws Exception{
  var fixture=new RootFlowDeletionRuntimeIT();fixture.setup();
  if("jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture".equals(System.getenv("B3_TEST_JDBC_URL"))){
   Runtime.getRuntime().addShutdownHook(new Thread(()->{try{fixture.cleanup();}catch(Exception e){throw new IllegalStateException("fixture cleanup failed");}}));
   java.nio.file.Files.writeString(java.nio.file.Path.of("/tmp/weaveos-v067-deletion-ready"),Integer.toString(fixture.port));
   new java.util.concurrent.CountDownLatch(1).await();
  }else{
   try{System.out.println("V067_READY="+fixture.port);System.out.flush();System.in.read();}finally{fixture.cleanup();}
  }
 }
}
