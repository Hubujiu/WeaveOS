package org.weaveos.workflow;

import static org.junit.jupiter.api.Assertions.*;
import com.google.protobuf.ByteString;
import io.grpc.*;
import io.grpc.health.v1.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.stub.MetadataUtils;
import java.nio.file.*;
import java.util.*;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.*;
import org.weaveos.workflow.v1.*;

/** Launches the actual main in a separate JVM, never a Root fixture main. */
class RootWorkflowMainProcessTest {
 final RootWorkflowRuntimeTest fixture=new RootWorkflowRuntimeTest();
 Process process;Path output;ManagedChannel channel;String caseName;
 @BeforeEach void setup(TestInfo info)throws Exception{fixture.setup();caseName=info.getTestMethod().orElseThrow().getName();output=Files.createTempFile("v041-main-", ".log");}
 @AfterEach void cleanup()throws Exception{
  try{
   if(channel!=null){channel.shutdownNow();channel.awaitTermination(5,TimeUnit.SECONDS);}
   if(process!=null&&process.isAlive()){process.destroy();if(!process.waitFor(10,TimeUnit.SECONDS)){process.destroyForcibly();process.waitFor(5,TimeUnit.SECONDS);}}
  }finally{
   try{
    if(output!=null&&Files.exists(output)){
     String safe=Files.readString(output).replace(RootWorkflowRuntimeTest.TOKEN,"[REDACTED synthetic token]").replace("v041_synthetic_only","[REDACTED synthetic password]").replace("private-argument-secret","[REDACTED synthetic argument]");
     Path evidence=Path.of("ci-logs/runtime-main-child");Files.createDirectories(evidence);Files.writeString(evidence.resolve(caseName+".log"),safe);
    }
   }finally{fixture.cleanup();if(output!=null)Files.deleteIfExists(output);}
  }
 }
 void launch(Map<String,String> values,String...args)throws Exception{
  List<String> command=new ArrayList<>(List.of(Path.of(System.getProperty("java.home"),"bin","java").toString(),"-cp",System.getProperty("java.class.path"),"org.weaveos.workflow.WorkflowEngineMain"));// Local isolated runners may provide a JVM-only hosts map; retain it in this child too.
  String hosts=System.getProperty("jdk.net.hosts.file");if(hosts!=null)command.add(1,"-Djdk.net.hosts.file="+hosts);
  command.addAll(Arrays.asList(args));
  var builder=new ProcessBuilder(command);builder.environment().clear();builder.environment().putAll(values);
  builder.environment().put("LANG","C.UTF-8");builder.redirectErrorStream(true);builder.redirectOutput(output.toFile());process=builder.start();
 }
 Channel authorized(){Metadata headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+RootWorkflowRuntimeTest.TOKEN);return ClientInterceptors.intercept(channel,MetadataUtils.newAttachHeadersInterceptor(headers));}
 void assertNoSecrets()throws Exception{String log=Files.readString(output);assertFalse(log.contains(RootWorkflowRuntimeTest.TOKEN));assertFalse(log.contains("v041_synthetic_only"));assertFalse(log.contains("private-argument-secret"));}
 @Test void actualMainServesThenSigtermPreservesCommittedDefinition()throws Exception{
  launch(fixture.env);channel=NettyChannelBuilder.forAddress("127.0.0.1",fixture.port).usePlaintext().disableRetry().build();
  long deadline=System.nanoTime()+20_000_000_000L;boolean ready=false;
  do{
   assertTrue(process.isAlive(),"formal main exited before serving");
   try{ready=HealthGrpc.newBlockingStub(authorized()).withDeadlineAfter(500,TimeUnit.MILLISECONDS).check(HealthCheckRequest.getDefaultInstance()).getStatus()==HealthCheckResponse.ServingStatus.SERVING;}catch(StatusRuntimeException ignored){}
   if(ready)break;Thread.sleep(25);
  }while(System.nanoTime()<deadline);
  assertTrue(ready,"formal main did not serve authenticated health");
  byte[] xml;try(var in=getClass().getResourceAsStream("/return-compatibility/return-all.bpmn20.xml")){assertNotNull(in);xml=in.readAllBytes();}
  var receipt=DeploymentServiceGrpc.newBlockingStub(authorized()).withDeadlineAfter(10,TimeUnit.SECONDS).deploy(DeployRequest.newBuilder().setAppId(RootExecutionRegistryTest.APP).setFlowId(RootExecutionRegistryTest.FLOW).setVersionId(RootExecutionRegistryTest.VERSION).setVersion(1).setBpmnXml(ByteString.copyFrom(xml)).build());
  assertFalse(receipt.getProcessDefinitionId().isEmpty());process.destroy();assertTrue(process.waitFor(10,TimeUnit.SECONDS),"SIGTERM did not finish bounded shutdown");fixture.noConnections();fixture.portAvailable();
  assertEquals(1,fixture.database.fixture.jdbc.queryForObject("SELECT count(*) FROM wf_deployments WHERE status='confirmed'",Integer.class));assertNoSecrets();
 }
 @Test void invalidEnvironmentExitsNonzeroWithoutSecretOutput()throws Exception{
  var values=new HashMap<>(fixture.env);values.remove("WEAVEOS_ENGINE_SERVICE_TOKEN");launch(values);assertTrue(process.waitFor(10,TimeUnit.SECONDS));assertNotEquals(0,process.exitValue());
  assertTrue(Files.readString(output).contains("workflow runtime initialization failed"));assertNoSecrets();fixture.noConnections();fixture.portAvailable();
 }
 @Test void unknownArgumentsFailWithoutEchoingArgument()throws Exception{
  launch(fixture.env,"--private-argument-secret");assertTrue(process.waitFor(10,TimeUnit.SECONDS));assertNotEquals(0,process.exitValue());
  assertTrue(Files.readString(output).contains("workflow runtime initialization failed"));assertNoSecrets();fixture.noConnections();fixture.portAvailable();
 }
}
