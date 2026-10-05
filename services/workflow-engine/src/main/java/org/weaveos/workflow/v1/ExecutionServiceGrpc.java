package org.weaveos.workflow.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ExecutionServiceGrpc {

  private ExecutionServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "weaveos.workflow.v1.ExecutionService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionRequest,
      org.weaveos.workflow.v1.ExecutionReceipt> getExecuteMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Execute",
      requestType = org.weaveos.workflow.v1.ExecutionRequest.class,
      responseType = org.weaveos.workflow.v1.ExecutionReceipt.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionRequest,
      org.weaveos.workflow.v1.ExecutionReceipt> getExecuteMethod() {
    io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionRequest, org.weaveos.workflow.v1.ExecutionReceipt> getExecuteMethod;
    if ((getExecuteMethod = ExecutionServiceGrpc.getExecuteMethod) == null) {
      synchronized (ExecutionServiceGrpc.class) {
        if ((getExecuteMethod = ExecutionServiceGrpc.getExecuteMethod) == null) {
          ExecutionServiceGrpc.getExecuteMethod = getExecuteMethod =
              io.grpc.MethodDescriptor.<org.weaveos.workflow.v1.ExecutionRequest, org.weaveos.workflow.v1.ExecutionReceipt>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Execute"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.ExecutionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.ExecutionReceipt.getDefaultInstance()))
              .setSchemaDescriptor(new ExecutionServiceMethodDescriptorSupplier("Execute"))
              .build();
        }
      }
    }
    return getExecuteMethod;
  }

  private static volatile io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionLookupRequest,
      org.weaveos.workflow.v1.ExecutionLookupResponse> getLookupMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Lookup",
      requestType = org.weaveos.workflow.v1.ExecutionLookupRequest.class,
      responseType = org.weaveos.workflow.v1.ExecutionLookupResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionLookupRequest,
      org.weaveos.workflow.v1.ExecutionLookupResponse> getLookupMethod() {
    io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionLookupRequest, org.weaveos.workflow.v1.ExecutionLookupResponse> getLookupMethod;
    if ((getLookupMethod = ExecutionServiceGrpc.getLookupMethod) == null) {
      synchronized (ExecutionServiceGrpc.class) {
        if ((getLookupMethod = ExecutionServiceGrpc.getLookupMethod) == null) {
          ExecutionServiceGrpc.getLookupMethod = getLookupMethod =
              io.grpc.MethodDescriptor.<org.weaveos.workflow.v1.ExecutionLookupRequest, org.weaveos.workflow.v1.ExecutionLookupResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Lookup"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.ExecutionLookupRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.ExecutionLookupResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ExecutionServiceMethodDescriptorSupplier("Lookup"))
              .build();
        }
      }
    }
    return getLookupMethod;
  }

  private static volatile io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionRequest,
      org.weaveos.workflow.v1.ExecutionReceipt> getEstablishNoEffectMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "EstablishNoEffect",
      requestType = org.weaveos.workflow.v1.ExecutionRequest.class,
      responseType = org.weaveos.workflow.v1.ExecutionReceipt.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionRequest,
      org.weaveos.workflow.v1.ExecutionReceipt> getEstablishNoEffectMethod() {
    io.grpc.MethodDescriptor<org.weaveos.workflow.v1.ExecutionRequest, org.weaveos.workflow.v1.ExecutionReceipt> getEstablishNoEffectMethod;
    if ((getEstablishNoEffectMethod = ExecutionServiceGrpc.getEstablishNoEffectMethod) == null) {
      synchronized (ExecutionServiceGrpc.class) {
        if ((getEstablishNoEffectMethod = ExecutionServiceGrpc.getEstablishNoEffectMethod) == null) {
          ExecutionServiceGrpc.getEstablishNoEffectMethod = getEstablishNoEffectMethod =
              io.grpc.MethodDescriptor.<org.weaveos.workflow.v1.ExecutionRequest, org.weaveos.workflow.v1.ExecutionReceipt>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "EstablishNoEffect"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.ExecutionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.weaveos.workflow.v1.ExecutionReceipt.getDefaultInstance()))
              .setSchemaDescriptor(new ExecutionServiceMethodDescriptorSupplier("EstablishNoEffect"))
              .build();
        }
      }
    }
    return getEstablishNoEffectMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ExecutionServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceStub>() {
        @java.lang.Override
        public ExecutionServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ExecutionServiceStub(channel, callOptions);
        }
      };
    return ExecutionServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ExecutionServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceBlockingV2Stub>() {
        @java.lang.Override
        public ExecutionServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ExecutionServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ExecutionServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ExecutionServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceBlockingStub>() {
        @java.lang.Override
        public ExecutionServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ExecutionServiceBlockingStub(channel, callOptions);
        }
      };
    return ExecutionServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ExecutionServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ExecutionServiceFutureStub>() {
        @java.lang.Override
        public ExecutionServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ExecutionServiceFutureStub(channel, callOptions);
        }
      };
    return ExecutionServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void execute(org.weaveos.workflow.v1.ExecutionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionReceipt> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getExecuteMethod(), responseObserver);
    }

    /**
     */
    default void lookup(org.weaveos.workflow.v1.ExecutionLookupRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionLookupResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getLookupMethod(), responseObserver);
    }

    /**
     */
    default void establishNoEffect(org.weaveos.workflow.v1.ExecutionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionReceipt> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getEstablishNoEffectMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ExecutionService.
   */
  public static abstract class ExecutionServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ExecutionServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ExecutionService.
   */
  public static final class ExecutionServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ExecutionServiceStub> {
    private ExecutionServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ExecutionServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ExecutionServiceStub(channel, callOptions);
    }

    /**
     */
    public void execute(org.weaveos.workflow.v1.ExecutionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionReceipt> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getExecuteMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void lookup(org.weaveos.workflow.v1.ExecutionLookupRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionLookupResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getLookupMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void establishNoEffect(org.weaveos.workflow.v1.ExecutionRequest request,
        io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionReceipt> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getEstablishNoEffectMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ExecutionService.
   */
  public static final class ExecutionServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ExecutionServiceBlockingV2Stub> {
    private ExecutionServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ExecutionServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ExecutionServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public org.weaveos.workflow.v1.ExecutionReceipt execute(org.weaveos.workflow.v1.ExecutionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getExecuteMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.ExecutionLookupResponse lookup(org.weaveos.workflow.v1.ExecutionLookupRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getLookupMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.ExecutionReceipt establishNoEffect(org.weaveos.workflow.v1.ExecutionRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getEstablishNoEffectMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ExecutionService.
   */
  public static final class ExecutionServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ExecutionServiceBlockingStub> {
    private ExecutionServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ExecutionServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ExecutionServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public org.weaveos.workflow.v1.ExecutionReceipt execute(org.weaveos.workflow.v1.ExecutionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getExecuteMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.ExecutionLookupResponse lookup(org.weaveos.workflow.v1.ExecutionLookupRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getLookupMethod(), getCallOptions(), request);
    }

    /**
     */
    public org.weaveos.workflow.v1.ExecutionReceipt establishNoEffect(org.weaveos.workflow.v1.ExecutionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getEstablishNoEffectMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ExecutionService.
   */
  public static final class ExecutionServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ExecutionServiceFutureStub> {
    private ExecutionServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ExecutionServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ExecutionServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<org.weaveos.workflow.v1.ExecutionReceipt> execute(
        org.weaveos.workflow.v1.ExecutionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getExecuteMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<org.weaveos.workflow.v1.ExecutionLookupResponse> lookup(
        org.weaveos.workflow.v1.ExecutionLookupRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getLookupMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<org.weaveos.workflow.v1.ExecutionReceipt> establishNoEffect(
        org.weaveos.workflow.v1.ExecutionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getEstablishNoEffectMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_EXECUTE = 0;
  private static final int METHODID_LOOKUP = 1;
  private static final int METHODID_ESTABLISH_NO_EFFECT = 2;

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
        case METHODID_EXECUTE:
          serviceImpl.execute((org.weaveos.workflow.v1.ExecutionRequest) request,
              (io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionReceipt>) responseObserver);
          break;
        case METHODID_LOOKUP:
          serviceImpl.lookup((org.weaveos.workflow.v1.ExecutionLookupRequest) request,
              (io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionLookupResponse>) responseObserver);
          break;
        case METHODID_ESTABLISH_NO_EFFECT:
          serviceImpl.establishNoEffect((org.weaveos.workflow.v1.ExecutionRequest) request,
              (io.grpc.stub.StreamObserver<org.weaveos.workflow.v1.ExecutionReceipt>) responseObserver);
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
          getExecuteMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              org.weaveos.workflow.v1.ExecutionRequest,
              org.weaveos.workflow.v1.ExecutionReceipt>(
                service, METHODID_EXECUTE)))
        .addMethod(
          getLookupMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              org.weaveos.workflow.v1.ExecutionLookupRequest,
              org.weaveos.workflow.v1.ExecutionLookupResponse>(
                service, METHODID_LOOKUP)))
        .addMethod(
          getEstablishNoEffectMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              org.weaveos.workflow.v1.ExecutionRequest,
              org.weaveos.workflow.v1.ExecutionReceipt>(
                service, METHODID_ESTABLISH_NO_EFFECT)))
        .build();
  }

  private static abstract class ExecutionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ExecutionServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return org.weaveos.workflow.v1.Execution.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ExecutionService");
    }
  }

  private static final class ExecutionServiceFileDescriptorSupplier
      extends ExecutionServiceBaseDescriptorSupplier {
    ExecutionServiceFileDescriptorSupplier() {}
  }

  private static final class ExecutionServiceMethodDescriptorSupplier
      extends ExecutionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ExecutionServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ExecutionServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ExecutionServiceFileDescriptorSupplier())
              .addMethod(getExecuteMethod())
              .addMethod(getLookupMethod())
              .addMethod(getEstablishNoEffectMethod())
              .build();
        }
      }
    }
    return result;
  }
}
