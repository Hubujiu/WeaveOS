package org.weaveos.workflow;

/** Declaration only for Root-first behavior tests. */
public final class CommandEnvelope {
 private CommandEnvelope() {}
 public static CommandEnvelope decode(byte[] bytes) { throw new IllegalArgumentException("not implemented"); }
 public int protocolVersion() { return 0; }
 public String commandId() { return null; }
 public String appId() { return null; }
 public String tableId() { return null; }
 public String viewId() { return null; }
 public String recordId() { return null; }
 public String flowId() { return null; }
 public String versionId() { return null; }
 public String instanceId() { return null; }
 public String taskId() { return null; }
 public String actorId() { return null; }
 public String action() { return null; }
 public String targetNodeId() { return null; }
 public long definitionVersion() { return 0; }
 public long schemaVersion() { return 0; }
 public long recordVersion() { return 0; }
 public long fenceEpoch() { return 0; }
 public long taskEpoch() { return 0; }
 public long expectedSequence() { return 0; }
 public byte[] payloadHash() { return null; }
 public byte[] canonicalBytes() { return null; }
 public byte[] fingerprint() { return null; }
}
