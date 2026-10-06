package org.weaveos.workflow;

import io.grpc.*;

/** Declaration-only stub for Root's authentication acceptance tests. */
public final class ServiceTokenInterceptor implements ServerInterceptor {
 public ServiceTokenInterceptor(String token) {}
 @Override public <Q,S> ServerCall.Listener<Q> interceptCall(ServerCall<Q,S> call, Metadata headers, ServerCallHandler<Q,S> next) {
  return next.startCall(call,headers);
 }
}
