# Release Notes and Roadmap

## Release Note

### v1.1.0

* 修改Service Slot定义格式
* 升级GormPlugin为DatabasePlugin，支持多数据库连接
* 升级DataService，支持分表存储及查询，支持视图查询
* 其他修改

## Roadmap

### Pipeline

* 支持配置Server RPCX序列化方式 none/json/protobuf/thrift
* 分布式事务（DT）
* 配套AdminWeb 服务
    - 服务管理
    - 服务监视（集成grafana）
* 应用安全监控与防御
* 创建分布式临界资源分配服务 GlobalResourceService
* 多级状态机
    - 使用mysql
    - 支持状态依赖
    - 支持状态分级
    - 支持状态自动合并
    - 死循环检测
* 添加待签名API类型（partially done）
* 集群化能力
    - 增加一个反代，来实现http-rpcx的负载均衡，同时支持反代的集群部署
    - 全局缓存
    - 全局任务分发
    - 全局日志
* 更好的服务监测与治理
    - 自动工具，集成 grafana？
    - 服务治理的支撑功能可以划分为三个层次：治理支撑服务，功能支撑服务，线下支撑服务。
        - 治理支撑服务包括服务注册/发现，流量控制，容错熔断，服务升级/回滚，链路跟踪，路由分发，超时重试，智能恢复等支撑工具集成。
        - 功能支撑服务包括监控告警，日志服务，认证鉴权，计量计费，消息服务，负载均衡，持久化服务，网管服务等支撑工具。
        - 线下支撑服务包括DevOps流程支撑服务，运行环境支撑。
* 更好的发布支持
    - docker打包
* 更好的自动文档支持
    - 基于api.json的自动接口文档支持
* 更好的数据库支持
    - 增加对Oracle DB的支持
* 更好的自动化测试工具
    - 联通性测试
    - 压力测试
    - 功能测试

### Done
* slots参数定义对象化，以取代现在的json文件 @ 2022.5
* 基于redis的路由机制 @2022.5
* 支持API版本号 @2021.6
* 支持服务对插件的依赖声明和检查 @2021.6
* 支持埋点 @2021.6
    - 改进middleware，支持指明是indoor,outdoor,以及inoutdoor的middleware(done)
    - 需要在一次请求中增加一个流水号，已保证in/out middleware时能够识别是不是同一个请求(done)
* 服务限流 @2021.6
    - 通过配置，可以指定针对指定api进行限流，也可以统一对服务请求进行限流
* 服务器熔断 @2021.6
    - 通过配置，可以指定对service、ip、用户进行熔断控制
* connector支持多packer @2021.7
* fileservice支持minio存取 @2021.10