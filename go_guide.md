# Go 编码规范

> [uber-go 编码规范](https://github.com/xxjwxc/uber_go_guide_cn)，主要用于对自己编码风格的提升

## 指导原则

### interface

1. 不对数据做修改的情况，不要用指针指向传递。值传递本质上还是指针的
2. 合理性验证，在编译阶段终止

```golang
// bad case
// 如果 Handler 没有实现 http.Handler，会在运行时报错
type Handler struct {
  // ...
}
func (h *Handler) ServeHTTP(
  w http.ResponseWriter,
  r *http.Request,
) {
  ...
}

//good case
type Handler struct {
  // ...
}
// 用于触发编译期的接口的合理性检查机制
// 如果 Handler 没有实现 http.Handler，会在编译期报错
var _ http.Handler = (*Handler)(nil)
func (h *Handler) ServeHTTP(
  w http.ResponseWriter,
  r *http.Request,
) {
  // ...
}
```

### receiver & 接口

interface 的值实现，用值或者指针都能调用
interface 的指针实现，只能指针调用，不能值调用

### 零值 Mutex 有效

不需要 Mutex 指针，在 struct 中要显式声明，不要直接匿名带入 sync.Mutex, 会导出 Lock 和 Unlock 方法

### 在边界处拷贝 Slices 和 Maps

在边界处 copy 过来

```golang
// bad case
type Stats struct {
  mu sync.Mutex

  counters map[string]int
}

// Snapshot 返回当前状态。
func (s *Stats) Snapshot() map[string]int {
  s.mu.Lock()
  defer s.mu.Unlock()

  return s.counters
}

// snapshot 不再受互斥锁保护
// 因此对 snapshot 的任何访问都将受到数据竞争的影响
// 影响 stats.counters
snapshot := stats.Snapshot()


// good case
type Stats struct {
  mu sync.Mutex

  counters map[string]int
}

func (s *Stats) Snapshot() map[string]int {
  s.mu.Lock()
  defer s.mu.Unlock()

  result := make(map[string]int, len(s.counters))
  for k, v := range s.counters {
    result[k] = v
  }
  return result
}

// snapshot 现在是一个拷贝
snapshot := stats.Snapshot()
```

### 使用 defer 释放资源

可读性好，开支小（除非是纳秒级程序）

### channel size 应该是 0 或者 1

0 无缓冲，1 按照原文意思是需要界定通道边界，竞态条件，以及逻辑上下文梳理

> 这个有点疑惑

### 时间处理

应该始终使用 time.Time，更加安全，而不是使用 int 比较或者其他的处理方式

传递时间的时候使用 duration，显式展现时间，如果必须要传递，则使用 int 或者 float64 时，则需要参数的名字里显式带上单位

### Errors

如果需要错误匹配，需要导出 errors.New 中的内容，否则就隐式字段。
对于动态的情况，使用 `fmt.Errorf`

error 的名字不要带 `failed` 等标识，这会在堆栈中层层累加

导出或者不导出都使用 Err 或者 err 开头

### 处理断言失败

### 不要使用 panic

### 使用原子操作

### 避免可变的全局变量

可变的情况下依赖注入，不要全局

### 避免直接结构体内嵌

最小暴露，继承或者内嵌会暴露和继承这些内嵌的结构。对用户没有任何影响的情况下可以使用，另外注意 sync.Mutex 不要内嵌

### 避免使用内置名称

### 避免使用 init

某些情况下必要的，例如

- 不能表示为单个赋值的复杂表达式。
- 可插入的钩子，如 database/sql、编码类型注册表等。
- 对 Google Cloud Functions 和其他形式的确定性预计算的优化。

### 追加时优先指定切片容量

指定 CAP，减少扩容，增加可读性

### 主函数退出方式 (Exit)

用 os.Exit 和 log.Fatal，不要直接 panic

且让 main 程序尽可能少的存在逻辑退出的情况，将逻辑打包至函数，变成可测试的

## 性能

### 优先 strconv 而不是 fmt

### 避免反复字符串到字节的转换

```golang
// bad case
for i := 0; i < b.N; i++ {
  w.Write([]byte("Hello world"))
}

// good case
data := []byte("Hello world")
for i := 0; i < b.N; i++ {
  w.Write(data)
}
```

### 指定容器容量

make map 的时候指定分配（知道大小的情况），防止扩容超出分配

make slice 的时候指定 cap, 减少大量扩容的时间（看 benchmark 差很多，2.48s 和 0.21s）

## 规范

> 仅记录需要注意的

### 包的命名

- 全部小写。没有大写或下划线。
- 大多数使用命名导入的情况下，不需要重命名。
- 简短而简洁。请记住，在每个使用的地方都完整标识了该名称。
- 不用复数。例如 net/url，而不是 net/urls。
- 不要用“common”，“util”，“shared”或“lib”。这些是不好的，信息量不足的名称。

### 函数顺序和分组

- 函数应按粗略的调用顺序排序。
- 同一文件中的函数应按接收者分组。

因此，导出的函数应先出现在文件中，放在 struct, const, var 定义的后面。
在定义类型之后，但在接收者的其余方法之前，可能会出现一个 newXYZ()/NewXYZ()
由于函数是按接收者分组的，因此普通工具函数应在文件末尾出现。

### 减少嵌套

### 对于未导出的顶层常量和变量，使用\_作为前缀（除了 err）

### 空 slice 等于 nil

### 减少变量作用域防止冲突

### 避免参数语义不明确

```golang
// func printInfo(name string, isLocal, done bool)
printInfo("foo", true /* isLocal */, true /* done */)
```

更好的方式是：

```golang
type Region int

const (
  UnknownRegion Region = iota
  Local
)

type Status int

const (
  StatusReady Status= iota + 1
  StatusDone
  // Maybe we will have a StatusInProgress in the future.
)

func printInfo(name string, region Region, status Status)
```

### 功能选择

```golang
// bad case
// package db

func Open(
  addr string,
  cache bool,
  logger *zap.Logger
) (*Connection, error) {
  // ...
}


// good case
// package db

type Option interface {
  // ...
}

func WithCache(c bool) Option {
  // ...
}

func WithLogger(log *zap.Logger) Option {
  // ...
}

// Open creates a connection.
func Open(
  addr string,
  opts ...Option,
) (*Connection, error) {
  // ...
}
```

或者最好是

```golang
type options struct {
  cache  bool
  logger *zap.Logger
}

type Option interface {
  apply(*options)
}

type cacheOption bool

func (c cacheOption) apply(opts *options) {
  opts.cache = bool(c)
}

func WithCache(c bool) Option {
  return cacheOption(c)
}

type loggerOption struct {
  Log *zap.Logger
}

func (l loggerOption) apply(opts *options) {
  opts.logger = l.Log
}

func WithLogger(log *zap.Logger) Option {
  return loggerOption{Log: log}
}

// Open creates a connection.
func Open(
  addr string,
  opts ...Option,
) (*Connection, error) {
  options := options{
    cache:  defaultCache,
    logger: zap.NewNop(),
  }

  for _, o := range opts {
    o.apply(&options)
  }

  // ...
}
```
