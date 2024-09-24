package main
// 简单代码实现
import (
    "log"
    "net/http"
    "time"

    "github.com/hashicorp/consul/api"
    pb "github.com/micro/go-micro/v2/registry"
    "github.com/micro/go-micro/v2/selector"
    "go-micro/plugins/registry/consul/v4"
)

// 服务注册插件接口
type ServiceRegistrationPlugin interface {
    Register() error
}

// 服务发现插件接口
type ServiceDiscoveryPlugin interface {
    Discover(serviceName string) (*pb.Service, error)
}

// 健康检查插件接口
type HealthCheckPlugin interface {
    Check(w http.ResponseWriter, r *http.Request)
}

type ConsulRegistrationPlugin struct {
    serviceID   string
    serviceName string
    address     string
    port        int
}

func (c *ConsulRegistrationPlugin) Register() error {
    config := api.DefaultConfig()
    client, err := api.NewClient(config)
    if err!= nil {
       return fmt.Errorf("Error creating Consul client: %v", err)
    }

    registration := &api.AgentServiceRegistration{
       ID:      c.serviceID,
       Name:    c.serviceName,
       Address: c.address,
       Port:    c.port,
       Check: &api.AgentServiceCheck{
          HTTP:     fmt.Sprintf("http://%s:%d/health", c.address, c.port),
          Interval: "10s",
          Timeout:  "5s",
       },
    }

    return client.Agent().ServiceRegister(registration)
}

type ConsulDiscoveryPlugin struct {
    registry *consul.Registry
}

func NewConsulDiscoveryPlugin() *ConsulDiscoveryPlugin {
    return &ConsulDiscoveryPlugin{registry: consul.NewRegistry()}
}

func (c *ConsulDiscoveryPlugin) Discover(serviceName string) (*pb.Service, error) {
    services, err := c.registry.GetService(serviceName)
    if err!= nil {
       return nil, fmt.Errorf("Error discovering service: %v", err)
    }
    return services[0], nil
}

type ConsulHealthCheckPlugin struct{}

func (c *ConsulHealthCheckPlugin) Check(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("OK"))
}

func makeRequest(serviceName string) {
    discoveryPlugin := NewConsulDiscoveryPlugin()
    service, err := discoveryPlugin.Discover(serviceName)
    if err!= nil {
       log.Fatalf("Error discovering service: %v", err)
    }

    next := selector.Random(service)
    node, err := next()
    if err!= nil {
       log.Fatalf("Error selecting node: %v", err)
    }

    url := fmt.Sprintf("http://%s:%d/your-endpoint", node.Address, node.Port)
    // 这里可以使用 http.Get 或其他方式发起请求到负载均衡后的节点
    // 为了演示，这里只是打印 URL
    fmt.Println("Making request to:", url)
}

func startHealthCheck(discoveryPlugin *ConsulDiscoveryPlugin) {
    // 设置连续失败次数阈值
    consecutiveFailureThreshold := 3
    // 设置时间窗口为 1 分钟
    timeWindow := 1 * time.Minute
    consecutiveFailures := make(map[string]int)
    lastCheckTimes := make(map[string]time.Time)

    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
       serviceName := "my-service-name"
       service, err := discoveryPlugin.Discover(serviceName)
       if err!= nil {
          log.Printf("Error discovering service: %v", err)
          continue
       }

       for _, node := range service.Nodes {
          url := fmt.Sprintf("http://%s:%d/health", node.Address, node.Port)
          resp, err := http.Get(url)
          isNetworkError := false
          if err!= nil {
             isNetworkError = true
          } else if resp.StatusCode!= http.StatusOK {
             // 非 200 状态码也可能是服务内部错误
             if resp.StatusCode!= http.StatusServiceUnavailable && resp.StatusCode!= http.StatusInternalServerError {
                isNetworkError = true
             }
          }

          if isNetworkError {
             consecutiveFailures[node.Id]++
             if consecutiveFailures[node.Id] < consecutiveFailureThreshold {
                // 小于阈值且在时间窗口内认为可能是网络抖动，不进行告警
                if time.Since(lastCheckTimes[node.Id]) <= timeWindow {
                   log.Printf("Possible network jitter. Service at %s:%d failed health check. Consecutive failures: %d", node.Address, node.Port, consecutiveFailures[node.Id])
                } else {
                   // 超出时间窗口，重置连续失败次数和时间
                   consecutiveFailures[node.Id] = 1
                   lastCheckTimes[node.Id] = time.Now()
                }
             } else {
                log.Printf("Service at %s:%d is not healthy. Consecutive failures: %d", node.Address, node.Port, consecutiveFailures[node.Id])
             }
          } else {
             consecutiveFailures[node.Id] = 0
             lastCheckTimes[node.Id] = time.Now()
             log.Printf("Service at %s:%d is healthy", node.Address, node.Port)
          }
       }
    }
}

func main() {
    // 使用 Consul 进行服务注册
    registrationPlugin := &ConsulRegistrationPlugin{
       serviceID:   "my-service",
       serviceName: "my-service-name",
       address:     "localhost",
       port:        8080,
    }
    err := registrationPlugin.Register()
    if err!= nil {
       log.Fatalf("Error registering service: %v", err)
    }

    // 设置健康检查路由
    healthCheckPlugin := &ConsulHealthCheckPlugin{}
    http.HandleFunc("/health", healthCheckPlugin.Check)
    go func() {
       log.Fatal(http.ListenAndServe(":8080", nil))
    }()

    discoveryPlugin := NewConsulDiscoveryPlugin()
    // 模拟根据服务名和路由请求时进行负载均衡
    makeRequest("my-service-name")

    // 启动定时健康检查
    go startHealthCheck(discoveryPlugin)

    // 防止主程序退出
    select {}
}
