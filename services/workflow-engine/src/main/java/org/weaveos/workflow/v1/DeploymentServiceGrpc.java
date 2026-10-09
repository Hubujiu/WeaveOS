package org.weaveos.workflow.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class DeploymentServiceGrpc {

  private DeploymentServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "weaveos.workflow.v1.DeploymentService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<org.weaveos.workflow.v1.DeployRequest,
      org.weaveos.workflow.v1.DeploymentReceipt> getDeployMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Deploy",
      requestType = org.weaveos.workflow.v1.DeployRequest.class,
      responseType = org.weaveos.workflow.v1.DeploymentReceipt.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<org.weaveos.workflow.v1.DeployRequest,
      org.weaveos.workflow.v1.DeploymentReceipt> getDeployMethod() {
    io.grpc.MethodDescriptor<org.weaveos.workflow.v1.DeployRequest, org.weaveos.workflow.v1.DeploymentReceipt> getDeployMethod;
    if ((getDeployMethod = DeploymentServiceGrpc.getDeployMethod) == null) {
      synchronized (DeploymentServiceGrpc.class) {
        if ((getDeployMethod = DeploymentServiceGrpc.getDeployMethod) == null) {
          DeploymentServiceGrpc.getDeployMethod = getDeployMethod =
              io.grpc.MethodDescriptor.<org.weaveos.workflow.v1.DeployRequest, org.weaveos.workflow.v1.DeploymentReceipt>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Deploy"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.DeployRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.DeploymentReceipt.getDefaultInstance()))
              .setSchemaDescriptor(new DeploymentServiceMethodDescriptorSupplier("Deploy"))
              .build();
        }
      }
    }
    return getDeployMethod;
  }

  private static volatile io.grpc.MethodDescriptor<org.weaveos.workflow.v1.LookupRequest,
      org.weaveos.workflow.v1.LookupResponse> getLookupMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Lookup",
      requestType = org.weaveos.workflow.v1.LookupRequest.class,
      responseType = org.weaveos.workflow.v1.LookupResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<org.weaveos.workflow.v1.LookupRequest,
      org.weaveos.workflow.v1.LookupResponse> getLookupMethod() {
    io.grpc.MethodDescriptor<org.weaveos.workflow.v1.LookupRequest, org.weaveos.workflow.v1.LookupResponse> getLookupMethod;
    if ((getLookupMethod = DeploymentServiceGrpc.getLookupMethod) == null) {
      synchronized (DeploymentServiceGrpc.class) {
        if ((getLookupMethod = DeploymentServiceGrpc.getLookupMethod) == null) {
          DeploymentServiceGrpc.getLookupMethod = getLookupMethod =
              io.grpc.MethodDescriptor.<org.weaveos.workflow.v1.LookupRequest, org.weaveos.workflow.v1.LookupResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Lookup"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.LookupRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.LookupResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DeploymentServiceMethodDescriptorSupplier("Lookup"))
              .build();
        }
      }
    }
    return getLookupMethod;
  }

  private static volatile io.grpc.MethodDescriptor<org.weaveos.workflow.v1.FlowDeletionRequest,
      org.weaveos.workflow.v1.FlowDeletionReceipt> getDeleteFlowMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteFlow",
      requestType = org.weaveos.workflow.v1.FlowDeletionRequest.class,
      responseType = org.weaveos.workflow.v1.FlowDeletionReceipt.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<org.weaveos.workflow.v1.FlowDeletionRequest,
      org.weaveos.workflow.v1.FlowDeletionReceipt> getDeleteFlowMethod() {
    io.grpc.MethodDescriptor<org.weaveos.workflow.v1.FlowDeletionRequest, org.weaveos.workflow.v1.FlowDeletionReceipt> getDeleteFlowMethod;
    if ((getDeleteFlowMethod = DeploymentServiceGrpc.getDeleteFlowMethod) == null) {
      synchronized (DeploymentServiceGrpc.class) {
        if ((getDeleteFlowMethod = DeploymentServiceGrpc.getDeleteFlowMethod) == null) {
          DeploymentServiceGrpc.getDeleteFlowMethod = getDeleteFlowMethod =
              io.grpc.MethodDescriptor.<org.weaveos.workflow.v1.FlowDeletionRequest, org.weaveos.workflow.v1.FlowDeletionReceipt>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteFlow"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.FlowDeletionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.FlowDeletionReceipt.getDefaultInstance()))
              .setSchemaDescriptor(new DeploymentServiceMethodDescriptorSupplier("DeleteFlow"))
              .build();
        }
      }
    }
    return getDeleteFlowMethod;
  }

  private static volatile io.grpc.MethodDescriptor<org.weaveos.workflow.v1.FlowDeletionRequest,
      org.weaveos.workflow.v1.FlowDeletionLookupResponse> getLookupFlowDeletionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "LookupFlowDeletion",
      requestType = org.weaveos.workflow.v1.FlowDeletionRequest.class,
      responseType = org.weaveos.workflow.v1.FlowDeletionLookupResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<org.weaveos.workflow.v1.FlowDeletionRequest,
      org.weaveos.workflow.v1.FlowDeletionLookupResponse> getLookupFlowDeletionMethod() {
    io.grpc.MethodDescriptor<org.weaveos.workflow.v1.FlowDeletionRequest, org.weaveos.workflow.v1.FlowDeletionLookupResponse> getLookupFlowDeletionMethod;
    if ((getLookupFlowDeletionMethod = DeploymentServiceGrpc.getLookupFlowDeletionMethod) == null) {
      synchronized (DeploymentServiceGrpc.class) {
        if ((getLookupFlowDeletionMethod = DeploymentServiceGrpc.getLookupFlowDeletionMethod) == null) {
          DeploymentServiceGrpc.getLookupFlowDeletionMethod = getLookupFlowDeletionMethod =
              io.grpc.MethodDescriptor.<org.weaveos.workflow.v1.FlowDeletionRequest, org.weaveos.workflow.v1.FlowDeletionLookupResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "LookupFlowDeletion"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.FlowDeletionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.FlowDeletionLookupResponse.getDefaultInstance()))
              .setSchemaDescriptor(new DeploymentServiceMethodDescriptorSupplier("LookupFlowDeletion"))
              .build();
        }
      }
    }
    return getLookupFlowDeletionMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static DeploymentServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceStub>() {
        @java.lang.Override
        public DeploymentServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DeploymentServiceStub(channel, callOptions);
        }
      };
    return DeploymentServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static DeploymentServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceBlockingV2Stub>() {
        @java.lang.Override
        public DeploymentServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DeploymentServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return DeploymentServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static DeploymentServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceBlockingStub>() {
        @java.lang.Override
        public DeploymentServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DeploymentServiceBlockingStub(channel, callOptions);
        }
      };
    return DeploymentServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static DeploymentServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<DeploymentServiceFutureStub>() {
        @java.lang.Override
        public DeploymentServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new DeploymentServiceFutureStub(channel, callOptions);
        }
      };
    return DeploymentServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void deploy(org.weaveos.workflow.v1.DeployRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.DeploymentReceipt> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeployMethod(), responseObserver);
    }

    /**
     */
    default void lookup(org.weaveos.workflow.v1.LookupRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.LookupResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getLookupMethod(), responseObserver);
    }

    /**
     */
    default void deleteFlow(org.weaveos.workflow.v1.FlowDeletionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.FlowDeletionReceipt> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteFlowMethod(), responseObserver);
    }

    /**
     */
    default void lookupFlowDeletion(org.weaveos.workflow.v1.FlowDeletionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.FlowDeletionLookupResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getLookupFlowDeletionMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service DeploymentService.
   */
  public static abstract class DeploymentServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return DeploymentServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service DeploymentService.
   */
  public static final class DeploymentServiceStub
      extends io.grpc.stub.AbstractAsyncStub<DeploymentServiceStub> {
    private DeploymentServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DeploymentServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DeploymentServiceStub(channel, callOptions);
    }

    /**
     */
    public void deploy(org.weaveos.workflow.v1.DeployRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.DeploymentReceipt> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeployMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void lookup(org.weaveos.workflow.v1.LookupRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.LookupResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getLookupMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void deleteFlow(org.weaveos.workflow.v1.FlowDeletionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.FlowDeletionReceipt> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteFlowMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void lookupFlowDeletion(org.weaveos.workflow.v1.FlowDeletionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.FlowDeletionLookupResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getLookupFlowDeletionMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service DeploymentService.
   */
  public static final class DeploymentServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<DeploymentServiceBlockingV2Stub> {
    private DeploymentServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DeploymentServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DeploymentServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public org.weaveos.workflow.v1.DeploymentReceipt deploy(org.weaveos.workflow.v1.DeployRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeployMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.LookupResponse lookup(org.weaveos.workflow.v1.LookupRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getLookupMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.FlowDeletionReceipt deleteFlow(org.weaveos.workflow.v1.FlowDeletionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getDeleteFlowMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.FlowDeletionLookupResponse lookupFlowDeletion(org.weaveos.workflow.v1.FlowDeletionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getLookupFlowDeletionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service DeploymentService.
   */
  public static final class DeploymentServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<DeploymentServiceBlockingStub> {
    private DeploymentServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DeploymentServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DeploymentServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public org.weaveos.workflow.v1.DeploymentReceipt deploy(org.weaveos.workflow.v1.DeployRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeployMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.LookupResponse lookup(org.weaveos.workflow.v1.LookupRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getLookupMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.FlowDeletionReceipt deleteFlow(org.weaveos.workflow.v1.FlowDeletionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteFlowMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.FlowDeletionLookupResponse lookupFlowDeletion(org.weaveos.workflow.v1.FlowDeletionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getLookupFlowDeletionMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service DeploymentService.
   */
  public static final class DeploymentServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<DeploymentServiceFutureStub> {
    private DeploymentServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected DeploymentServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new DeploymentServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<org.weaveos.workflow.v1.DeploymentReceipt> deploy(
        org.weaveos.workflow.v1.DeployRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeployMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<org.weaveos.workflow.v1.LookupResponse> lookup(
        org.weaveos.workflow.v1.LookupRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getLookupMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<org.weaveos.workflow.v1.FlowDeletionReceipt> deleteFlow(
        org.weaveos.workflow.v1.FlowDeletionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteFlowMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<org.weaveos.workflow.v1.FlowDeletionLookupResponse> lookupFlowDeletion(
        org.weaveos.workflow.v1.FlowDeletionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getLookupFlowDeletionMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_DEPLOY = 0;
  private static final int METHODID_LOOKUP = 1;
  private static final int METHODID_DELETE_FLOW = 2;
  private static final int METHODID_LOOKUP_FLOW_DELETION = 3;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_DEPLOY:
          serviceImpl.deploy((org.weaveos.workflow.v1.DeployRequest) request,
              (io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.DeploymentReceipt>) responseObserver);
          break;
        case METHODID_LOOKUP:
          serviceImpl.lookup((org.weaveos.workflow.v1.LookupRequest) request,
              (io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.LookupResponse>) responseObserver);
          break;
        case METHODID_DELETE_FLOW:
          serviceImpl.deleteFlow((org.weaveos.workflow.v1.FlowDeletionRequest) request,
              (io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.FlowDeletionReceipt>) responseObserver);
          break;
        case METHODID_LOOKUP_FLOW_DELETION:
          serviceImpl.lookupFlowDeletion((org.weaveos.workflow.v1.FlowDeletionRequest) request,
              (io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.FlowDeletionLookupResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getDeployMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              org.weaveos.workflow.v1.DeployRequest,
              org.weaveos.workflow.v1.DeploymentReceipt>(
                service, METHODID_DEPLOY)))
        .addMethod(
          getLookupMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              org.weaveos.workflow.v1.LookupRequest,
              org.weaveos.workflow.v1.LookupResponse>(
                service, METHODID_LOOKUP)))
        .addMethod(
          getDeleteFlowMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              org.weaveos.workflow.v1.FlowDeletionRequest,
              org.weaveos.workflow.v1.FlowDeletionReceipt>(
                service, METHODID_DELETE_FLOW)))
        .addMethod(
          getLookupFlowDeletionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              org.weaveos.workflow.v1.FlowDeletionRequest,
              org.weaveos.workflow.v1.FlowDeletionLookupResponse>(
                service, METHODID_LOOKUP_FLOW_DELETION)))
        .build();
  }

  private static abstract class DeploymentServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    DeploymentServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return org.weaveos.workflow.v1.Deployment.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("DeploymentService");
    }
  }

  private static final class DeploymentServiceFileDescriptorSupplier
      extends DeploymentServiceBaseDescriptorSupplier {
    DeploymentServiceFileDescriptorSupplier() {}
  }

  private static final class DeploymentServiceMethodDescriptorSupplier
      extends DeploymentServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    DeploymentServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (DeploymentServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new DeploymentServiceFileDescriptorSupplier())
              .addMethod(getDeployMethod())
              .addMethod(getLookupMethod())
              .addMethod(getDeleteFlowMethod())
              .addMethod(getLookupFlowDeletionMethod())
              .build();
        }
      }
    }
    return result;
  }
}
