package org.weaveos.workflow;

import org.springframework.jdbc.core.JdbcTemplate;

/** Called only inside the shared REQUIRED transaction; the row lock lasts through commit. */
final class FlowIdentityGate {
 private FlowIdentityGate() {}
 static boolean lock(JdbcTemplate jdbc, String app, String flow) {
  jdbc.update("INSERT INTO wf_flow_deletion_guards(app_id,flow_id) VALUES(?::uuid,?::uuid) ON CONFLICT DO NOTHING",app,flow);
  return Boolean.TRUE.equals(jdbc.queryForObject(
   "SELECT retired FROM wf_flow_deletion_guards WHERE app_id=?::uuid AND flow_id=?::uuid FOR UPDATE",Boolean.class,app,flow));
 }
}
