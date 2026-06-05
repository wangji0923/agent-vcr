# 03 - Command and Path Classifiers

## 目标

实现命令分类器和路径分类器，为事件抽取和行为指标提供稳定、可测试的语义判断。

## 允许修改

```text
internal/behavior/command_classifier.go
internal/behavior/path_classifier.go
internal/behavior/classifier_test.go
testdata/behavior/commands/**
testdata/behavior/paths/**
```

## 禁止修改

```text
internal/adapters/**
cmd/**
internal/trace/**
```

## CommandClassifier

建议接口：

```go
type CommandKind string

const (
    CommandUnknown           CommandKind = "unknown"
    CommandSearch            CommandKind = "search"
    CommandReadFile          CommandKind = "read_file"
    CommandRunTest           CommandKind = "run_test"
    CommandRunBuild          CommandKind = "run_build"
    CommandInstallDependency CommandKind = "install_dependency"
)

type CommandClassification struct {
    Kind       CommandKind
    Query      string
    Files      []string
    Command    string
    Confidence float64
    Attributes map[string]string
}

func ClassifyCommand(command string) CommandClassification
```

## Shell 命令识别规则

### Search

```text
rg "..." ...
grep -R "..." ...
fd "..."
find . -name ...
```

输出：

```text
Kind = search
Query = 搜索词，best effort
Files = 搜索 scope，best effort
```

### Test

```text
npm test
npm run test
pnpm test
yarn test
pytest
python -m pytest
go test ./...
cargo test
mvn test
gradle test
```

输出：

```text
Kind = run_test
```

### Build

```text
npm run build
pnpm build
go build
cargo build
mvn package
gradle build
```

### Dependency install

```text
npm install
npm i
pnpm add
yarn add
go get
cargo add
pip install
```

### Read file

```text
cat file
sed -n '1,100p' file
head file
tail file
```

## PathClassifier

建议接口：

```go
type PathKind string

const (
    PathUnknown PathKind = "unknown"
    PathSource  PathKind = "source"
    PathTest    PathKind = "test"
    PathConfig  PathKind = "config"
    PathDocs    PathKind = "docs"
    PathLegacy  PathKind = "legacy"
    PathSecret  PathKind = "secret"
)

func ClassifyPath(path string) PathKind
func IsTestPath(path string) bool
func IsLegacyPath(path string) bool
func IsSecretPath(path string) bool
func NormalizePath(path string) string
```

## 路径规则

### Test path

```text
tests/**
test/**
**/*_test.go
**/*.test.*
**/*.spec.*
__tests__/**
```

### Legacy path

```text
**/legacy/**
**/deprecated/**
**/*legacy*
**/*deprecated*
```

### Secret path

```text
.env
.env.*
secrets/**
**/id_rsa
**/*.pem
```

### Config path

```text
package.json
go.mod
Cargo.toml
pyproject.toml
*.config.*
tsconfig.json
```

## 测试要求

```text
TestClassifyCommandSearchRg
TestClassifyCommandSearchGrep
TestClassifyCommandRunTestNpm
TestClassifyCommandRunTestGo
TestClassifyCommandBuild
TestClassifyCommandInstallDependency
TestClassifyCommandReadFile
TestClassifyCommandUnknown
TestClassifyPathTest
TestClassifyPathLegacy
TestClassifyPathSecret
TestNormalizePathWindowsAndUnix
```

## 验收命令

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
```

## Codex 执行提示词

```text
请只实现 03-command-and-path-classifiers.md。实现命令分类和路径分类，不要接 CLI，不要改 trace schema，不要实现 divergence。补充表驱动测试，运行 go test ./internal/behavior/... 和 go test ./...。
```
