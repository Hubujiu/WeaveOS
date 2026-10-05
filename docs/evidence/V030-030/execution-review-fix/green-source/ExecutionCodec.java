package org.weaveos.workflow;

import java.io.ByteArrayOutputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.CharBuffer;
import java.nio.charset.StandardCharsets;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.*;
import org.weaveos.workflow.ExecutionRegistry.*;

/** Bounded internal framing. No expressions or arbitrary engine variables. */
final class ExecutionCodec {
    static final long MAX = 9007199254740991L;
    private static final Set<String> REASONS = Set.of("instance_missing", "instance_exists", "deployment_missing", "deployment_mismatch", "scope_mismatch", "terminal_instance", "stale_sequence", "stale_fence", "stale_schema_version", "stale_record_version", "task_missing", "task_inactive", "task_epoch_mismatch", "actor_mismatch", "withdrawal_forbidden", "return_target_unvisited", "return_target_forbidden", "cancelled");
    private static final byte[] PAY = {87,86,70,80,65,89,0,1};
    private static final byte[] RESULT = {87,86,70,82,83,76,0,1};
    record Payload(boolean start, boolean withdraw, Map<String,List<String>> rosters, Map<String,Boolean> routes) {
        Map<String,Object> variables(boolean includeRosters) {
            Map<String,Object> vars = new HashMap<>();
            routes.forEach((id,value) -> vars.put("route_" + compact(id), value));
            if (includeRosters) rosters.forEach((id,actors) -> vars.put("a_" + compact(id), actors));
            return vars;
        }
    }
    static Payload payload(byte[] bytes, boolean start) {
        if (bytes == null || bytes.length > 262144 || bytes.length < 45) throw new InvalidCommand();
        Reader r = new Reader(bytes);
        r.prefix(PAY); byte[] evidence = r.bytes(32);
        if (Arrays.equals(evidence,new byte[32])) throw new InvalidCommand();
        boolean config = r.flag();
        if (config != start) throw new InvalidCommand();
        boolean withdraw = config && r.flag();
        Map<String,List<String>> rosters = new TreeMap<>();
        if (config) {
            int n = r.count(100); String prev = "";
            for (int i=0;i<n;i++) {
                String id=r.uuid(); ascending(prev,id); prev=id;
                int count=r.count(50); if(count==0) throw new InvalidCommand();
                List<String> actors=new ArrayList<>(); String prior="";
                for(int j=0;j<count;j++){String actor=r.uuid();ascending(prior,actor);prior=actor;actors.add(actor);}
                rosters.put(id,List.copyOf(actors));
            }
        }
        Map<String,Boolean> routes=new TreeMap<>();int n=r.count(100);String prior="";
        for(int i=0;i<n;i++){String id=r.uuid();ascending(prior,id);prior=id;routes.put(id,r.flag());}
        r.end();return new Payload(config,withdraw,Map.copyOf(rosters),Map.copyOf(routes));
    }
    private static void ascending(String a,String b){if(a.compareTo(b)>=0)throw new InvalidCommand();}
    static String compact(String uuid){return uuid.replace("-","");}
    static String nodeId(String activity){
        if(activity==null || !activity.matches("n_[0-9a-f]{32}"))throw new IllegalStateException("unexpected engine task node");
        String s=activity.substring(2);return s.substring(0,8)+"-"+s.substring(8,12)+"-"+s.substring(12,16)+"-"+s.substring(16,20)+"-"+s.substring(20);
    }
    static void uuid(String id){
        if(id==null || !id.matches("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}") || id.equals("00000000-0000-0000-0000-000000000000"))throw new InvalidCommand();
    }
    static byte[] hash(byte[] bytes){try{return MessageDigest.getInstance("SHA-256").digest(bytes);}catch(NoSuchAlgorithmException e){throw new IllegalStateException(e);}}
    static String hex(byte[] bytes){return HexFormat.of().formatHex(bytes);}
    static byte[] encode(Result result){
        try{
            var buffer=new ByteArrayOutputStream();var out=new DataOutputStream(buffer);out.write(RESULT);
            text(out,result.instanceId());text(out,result.engineProcessId());text(out,result.state());text(out,result.reason());
            out.writeLong(result.schemaVersion());out.writeLong(result.recordVersion());out.writeInt(result.tasks().size());
            for(Task t:result.tasks()){text(out,t.id());text(out,t.nodeId());text(out,t.assigneeId());text(out,t.engineTaskId());out.writeLong(t.activationEpoch());}
            byte[] bytes=buffer.toByteArray();if(bytes.length>65536)throw new IllegalStateException("result too large");decodeResult(bytes);return bytes;
        }catch(IOException invalidText){throw new IllegalStateException("invalid durable execution result text");}
    }
    private static void text(DataOutputStream out,String s)throws IOException{
        ByteBuffer encoded=StandardCharsets.UTF_8.newEncoder()
            .onMalformedInput(CodingErrorAction.REPORT).onUnmappableCharacter(CodingErrorAction.REPORT)
            .encode(CharBuffer.wrap(s));
        byte[] bytes=new byte[encoded.remaining()];encoded.get(bytes);out.writeInt(bytes.length);out.write(bytes);
    }
    static Result decodeResult(byte[] bytes){
        try{
            if(bytes==null || bytes.length>65536)throw new InvalidCommand();
            Reader r=new Reader(bytes);r.prefix(RESULT);String instance=r.uuid(),process=r.text(200),state=r.text(20),reason=r.text(50);
            long schema=r.number(),record=r.number();if(schema==0 || record==0)throw new InvalidCommand();
            int n=r.count(50);List<Task> tasks=new ArrayList<>();String prior="";
            for(int i=0;i<n;i++){String id=r.uuid();ascending(prior,id);prior=id;String node=r.uuid(),actor=r.uuid(),engine=r.text(200);long epoch=r.number();if(engine.isEmpty()||epoch==0)throw new InvalidCommand();tasks.add(new Task(id,node,actor,engine,epoch));}
            r.end();
            if(state.equals("unchanged")){if(!REASONS.contains(reason)||!process.isEmpty()||n!=0)throw new InvalidCommand();}
            else if(!Set.of("active","completed","rejected","withdrawn").contains(state)||!reason.isEmpty()||process.isEmpty()||(state.equals("active")!=(n>0)))throw new InvalidCommand();
            return new Result(instance,process,state,reason,schema,record,tasks);
        }catch(InvalidCommand corrupt){throw new IllegalStateException("invalid durable execution result");}
    }
    private static final class Reader {
        final ByteBuffer b;Reader(byte[] bytes){b=ByteBuffer.wrap(bytes);}
        void require(int n){if(n<0||b.remaining()<n)throw new InvalidCommand();}
        byte[] bytes(int n){require(n);byte[] bytes=new byte[n];b.get(bytes);return bytes;}
        void prefix(byte[] prefix){if(!Arrays.equals(bytes(prefix.length),prefix))throw new InvalidCommand();}
        int count(int max){require(4);long n=Integer.toUnsignedLong(b.getInt());if(n>max)throw new InvalidCommand();return (int)n;}
        boolean flag(){require(1);byte v=b.get();if(v!=0&&v!=1)throw new InvalidCommand();return v==1;}
        String text(int max){
            int n=count(max);byte[] bytes=bytes(n);
            try{
                String text=StandardCharsets.UTF_8.newDecoder()
                    .onMalformedInput(CodingErrorAction.REPORT).onUnmappableCharacter(CodingErrorAction.REPORT)
                    .decode(ByteBuffer.wrap(bytes)).toString();
                if(text.codePoints().anyMatch(Character::isISOControl))throw new InvalidCommand();
                return text;
            }catch(CharacterCodingException malformed){throw new InvalidCommand();}
        }
        String uuid(){String s=text(36);ExecutionCodec.uuid(s);return s;}
        long number(){require(8);long n=b.getLong();if(n<0||n>MAX)throw new InvalidCommand();return n;}
        void end(){if(b.hasRemaining())throw new InvalidCommand();}
    }
}
