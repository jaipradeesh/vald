# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
import grpc

import payload_pb2 as payload__pb2


class EgressFilterStub(object):
  # missing associated documentation comment in .proto file
  pass

  def __init__(self, channel):
    """Constructor.

    Args:
      channel: A grpc.Channel.
    """
    self.Filter = channel.unary_unary(
        '/egress_filter.EgressFilter/Filter',
        request_serializer=payload__pb2.Search.Response.SerializeToString,
        response_deserializer=payload__pb2.Search.Response.FromString,
        )
    self.StreamFilter = channel.stream_stream(
        '/egress_filter.EgressFilter/StreamFilter',
        request_serializer=payload__pb2.Object.Distance.SerializeToString,
        response_deserializer=payload__pb2.Object.Distance.FromString,
        )


class EgressFilterServicer(object):
  # missing associated documentation comment in .proto file
  pass

  def Filter(self, request, context):
    # missing associated documentation comment in .proto file
    pass
    context.set_code(grpc.StatusCode.UNIMPLEMENTED)
    context.set_details('Method not implemented!')
    raise NotImplementedError('Method not implemented!')

  def StreamFilter(self, request_iterator, context):
    # missing associated documentation comment in .proto file
    pass
    context.set_code(grpc.StatusCode.UNIMPLEMENTED)
    context.set_details('Method not implemented!')
    raise NotImplementedError('Method not implemented!')


def add_EgressFilterServicer_to_server(servicer, server):
  rpc_method_handlers = {
      'Filter': grpc.unary_unary_rpc_method_handler(
          servicer.Filter,
          request_deserializer=payload__pb2.Search.Response.FromString,
          response_serializer=payload__pb2.Search.Response.SerializeToString,
      ),
      'StreamFilter': grpc.stream_stream_rpc_method_handler(
          servicer.StreamFilter,
          request_deserializer=payload__pb2.Object.Distance.FromString,
          response_serializer=payload__pb2.Object.Distance.SerializeToString,
      ),
  }
  generic_handler = grpc.method_handlers_generic_handler(
      'egress_filter.EgressFilter', rpc_method_handlers)
  server.add_generic_rpc_handlers((generic_handler,))
