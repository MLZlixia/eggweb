# eggweb
简单的服务注册和发现
# 技术选型
Consul 在实现服务注册与发现和健康检查方面具有以下优点：
 
一、服务注册与发现
 
1. 高可用性：
 
- Consul 采用分布式架构，支持多数据中心部署。这意味着即使部分节点出现故障，整个系统仍然可以正常运行，保证了服务注册与发现的高可用性。
- 服务可以在不同的数据中心进行注册，客户端可以从任何一个数据中心获取服务列表，提高了系统的容错能力。

2. 易于使用：
 
- Consul 提供了简单易用的 API 和命令行工具，使得服务的注册和发现变得非常容易。
- 在 Go 语言中，可以使用  github.com/hashicorp/consul/api  库轻松地与 Consul 进行交互，实现服务的注册、发现和健康检查。
  
3. 强大的服务发现功能：
 
- Consul 支持多种服务发现方式，包括 DNS 查找、HTTP API 和客户端库。这使得不同类型的客户端可以根据自己的需求选择合适的服务发现方式。
- 服务发现可以基于服务名称、标签或其他属性进行过滤和选择，提高了服务发现的灵活性和准确性。

4. 动态配置：
 
- Consul 支持动态配置，可以在运行时更新服务的配置信息。这使得服务可以根据不同的环境和需求进行动态配置，提高了系统的可扩展性和灵活性。
- 服务可以通过 Consul 的 Key/Value 存储功能获取配置信息，也可以通过 Consul 的 HTTP API 进行配置更新。
 
二、健康检查
 
1. 多种健康检查方式：
 
- Consul 支持多种健康检查方式，包括 HTTP、TCP、GRPC 和脚本检查等。这使得服务可以根据自己的需求选择合适的健康检查方式，提高了健康检查的灵活性和准确性。
- 健康检查可以设置检查间隔、超时时间和重试次数等参数，以适应不同的服务特性和网络环境。
2. 自动故障检测：
 
- Consul 会自动对注册的服务进行健康检查，并及时发现服务的故障。当服务出现故障时，Consul 会将该服务标记为不健康状态，并在进行服务发现时不返回不健康的服务实例，从而确保客户端不会被路由到有问题的服务上。
- 自动故障检测可以提高系统的可靠性和稳定性，减少因服务故障而导致的系统故障。
3. 健康状态可视化：
 
- Consul 提供了一个可视化的界面，可以查看服务的健康状态和统计信息。这使得管理员可以直观地了解系统的运行状态，及时发现和解决问题。
- 健康状态可视化可以提高系统的可维护性和管理效率，减少因系统故障而导致的损失。
 
总之，Consul 在实现服务注册与发现和健康检查方面具有高可用性、易于使用、强大的服务发现功能、动态配置、多种健康检查方式、自动故障检测和健康状态可视化等优点。这些优点使得 Consul 成为了一个非常流行的服务注册与发现和健康检查工具，被广泛应用于微服务架构和分布式系统中。

## 需要考虑问题
1 网络分区如何处理
  ```
  package main

import (
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/hashicorp/consul/api"
)

type Service struct {
    Name    string
    Address string
    Port    int
}

func registerService(client *api.Client, service Service) {
    registration := &api.AgentServiceRegistration{
       Name:    service.Name,
       Address: service.Address,
       Port:    service.Port,
       Check: &api.AgentServiceCheck{
          HTTP: fmt.Sprintf("http://%s:%d/health", service.Address, service.Port),
          Interval: "10s",
       },
    }
    err := client.Agent().ServiceRegister(registration)
    if err!= nil {
       log.Fatal(err)
    }
}

func discoverServices(client *api.Client) []*api.ServiceEntry {
    services, _, err := client.Health().Service("my-service", "", true, nil)
    if err!= nil {
       log.Println("Error discovering services:", err)
       return nil
    }
    return services
}

// 本地健康检查函数
func localHealthCheck(serviceAddress string) bool {
    resp, err := http.Get(fmt.Sprintf("http://%s/health", serviceAddress))
    if err!= nil {
       return false
    }
    defer resp.Body.Close()
    return resp.StatusCode == http.StatusOK
}

func main() {
    // 创建 Consul 配置
    config := api.DefaultConfig()

    // 创建 Consul 客户端
    client, err := api.NewClient(config)
    if err!= nil {
       log.Fatal(err)
    }

    // 注册服务
    myService := Service{
       Name:    "my-service",
       Address: "127.0.0.1",
       Port:    8080,
    }
    registerService(client, myService)

    // 本地缓存服务列表
    localServiceCache := make(map[string][]string)

    // 重试相关变量
    maxRetries := 5
    retryInterval := 3 * time.Second

    for {
       services := discoverServices(client)
       if services!= nil {
          addresses := make([]string, len(services))
          for i, service := range services {
             addresses[i] = fmt.Sprintf("%s:%d", service.Service.Address, service.Service.Port)
          }
          localServiceCache["my-service"] = addresses
       } else {
          // 如果无法从 Consul 获取服务列表，使用本地缓存并进行重试
          if cachedAddresses, ok := localServiceCache["my-service"]; ok {
             log.Println("Using local cache due to network partition.")
             for _, address := range cachedAddresses {
                if localHealthCheck(address) {
                   fmt.Println("Service at", address, "is healthy locally.")
                } else {
                   fmt.Println("Service at", address, "is not healthy locally.")
                }
             }

             retries := 0
             for retries < maxRetries {
                log.Println("Retrying to connect to Consul...")
                services = discoverServices(client)
                if services!= nil {
                   break
                }
                retries++
                time.Sleep(retryInterval)
             }
          }
       }

       time.Sleep(5 * time.Second)
    }
}
  ```
2 agent失恋的处理
3 发现服务时如何实现负载均衡
4 健康检查的配置
5 服务告警故障
