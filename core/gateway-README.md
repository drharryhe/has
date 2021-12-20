# apigateway

API 网关 可以提供Web Server 和其他类型的对外接口

## DEFINE

- ApiGateway不支持直接调用后端service（隐藏微服务）
- 使用yaml定义OpenAPI参数及验证
- ApiGateway同样支持多个connector，以支持不同通信方式
- 支持middleware
- packer 应该是相对固定的，因为要对应yaml定义
- 支持对api进行动态控制、健康监控
- apigateway 对后台服务有负载均衡
- 使用redis对api进行管理，未来可以扩展到nacos/redis等
- 为了保证apigateway的高效，网关上不实现业务逻辑，只完成api 到 service/slot的映射
- 对于简单后台服务，可以将service/slot直接注册到apigateway自带的server上
- 支持限流熔断等(注：has server本身已经支持熔断限流了)

## TODO

- 基于api描述文件的文档生成（描述参数尽量与swagger一致)
- 