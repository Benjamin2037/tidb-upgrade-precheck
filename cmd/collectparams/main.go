package main

import (
       "encoding/json"
       import (
              "encoding/json"
              "flag"
              "fmt"
              "os"
              "github.com/pingcap/tidb-upgrade-precheck/pkg/collectparams"
       )

       func main() {
              tidbRoot := flag.String("tidb", "../tidb", "TiDB 源码根目录")
              tag := flag.String("tag", "", "采集的 TiDB 版本 tag")
              out := flag.String("out", "", "输出文件路径")
              flag.Parse()

              if *tag == "" || *out == "" {
                     fmt.Println("需要 --tag 和 --out")
                     os.Exit(1)
              }

              snap, err := collectparams.CollectFromTidbSource(*tidbRoot, *tag)
              if err != nil {
                     fmt.Println("采集失败:", err)
                     os.Exit(1)
              }
              data, _ := json.MarshalIndent(snap, "", "  ")
              os.WriteFile(*out, data, 0644)
              fmt.Println("全量参数快照已生成:", *out)
       }

              return &ParamSnapshot{
                     Version:         tag,
                     ConfigDefaults:  configDefaults,
                     SystemVariables: sysVars,
              }, nil
// 递归解析 vardef 目录下所有 go 文件，提取常量名与值
func parseVardefConstants(vardefDir string) map[string]string {
       result := make(map[string]string)
       filepath.WalkDir(vardefDir, func(path string, d os.DirEntry, err error) error {
              if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
                     return nil
              }
              data, err := os.ReadFile(path)
              if err != nil {
                     return nil
              }
              // 匹配 const XXX = "..." 或 const XXX = 123
              re := regexp.MustCompile(`const\\s+([A-Za-z0-9_]+)\\s*=\\s*(["` + "'" + `]?)([^"'\\n]+)\\2`)
              matches := re.FindAllStringSubmatch(string(data), -1)
              for _, m := range matches {
                     if len(m) == 4 {
                            result[m[1]] = m[3]
                     }
              }
              return nil
       })
       return result
}
}

// 解析 config.go 静态默认值（简化示例，实际需完善）
func parseConfigDefaults(configPath string) map[string]interface{} {
       result := make(map[string]interface{})
       data, err := os.ReadFile(configPath)
       if err != nil {
	       return result
       }
       // 简单正则匹配 Config 结构体字段及默认值
       re := regexp.MustCompile(`([A-Za-z0-9_]+)\s+([A-Za-z0-9_]+)\s+` + "`.*default:(\\S+).*")
       lines := strings.Split(string(data), "\n")
       for _, line := range lines {
	       m := re.FindStringSubmatch(line)
	       if len(m) == 4 {
		       result[m[2]] = m[3]
	       }
       }
       return result
}

// 解析 sysvar.go 静态默认值（简化示例，实际需完善）
func parseSysVars(sysvarPath string) map[string]interface{} {
       result := make(map[string]interface{})
       data, err := os.ReadFile(sysvarPath)
       if err != nil {
	       return result
       }
       // 匹配 SysVar{Name: ..., Value: ...}
       re := regexp.MustCompile(`Name:\s*"([A-Za-z0-9_]+)",[\s\S]*?Value:\s*"([^"]*)"`)
       matches := re.FindAllStringSubmatch(string(data), -1)
       for _, m := range matches {
	       if len(m) == 3 {
		       result[m[1]] = m[2]
	       }
       }
       return result
}

func main() {
       tidbRoot := flag.String("tidb", "../tidb", "TiDB 源码根目录")
       tag := flag.String("tag", "", "采集的 TiDB 版本 tag")
       out := flag.String("out", "", "输出文件路径")
       flag.Parse()

       if *tag == "" || *out == "" {
              fmt.Println("需要 --tag 和 --out")
              os.Exit(1)
       }

       snap, err := collectparams.CollectFromTidbSource(*tidbRoot, *tag)
       if err != nil {
              fmt.Println("采集失败:", err)
              os.Exit(1)
       }
       data, _ := json.MarshalIndent(snap, "", "  ")
       os.WriteFile(*out, data, 0644)
       fmt.Println("全量参数快照已生成:", *out)
}























































}	os.Exit(1)	fmt.Println("未知模式:", *mode)	}		return		fmt.Println("增量 diff 逻辑待实现")		// TODO: 加载 base 快照和当前快照，输出差异		}			os.Exit(1)			fmt.Println("diff 模式需要 --base 和 --out")		if *base == "" || *out == "" {	if *mode == "diff" {	}		return		fmt.Println("全量参数快照已生成:", *out)		os.WriteFile(*out, data, 0644)		data, _ := json.MarshalIndent(snapshot, "", "  ")		}			SystemVariables: map[string]interface{}{}, // 待采集			ConfigDefaults:  map[string]interface{}{}, // 待采集			Version:         *ver,		snapshot := ParamSnapshot{		// TODO: 解析 TiDB 源码，采集全量 config 和 system vars 默认值		}			os.Exit(1)			fmt.Println("full 模式需要 --version 和 --out")		if *ver == "" || *out == "" {	if *mode == "full" {	flag.Parse()	out := flag.String("out", "", "输出文件路径")	base := flag.String("base", "", "diff 模式下的基线版本快照文件")	ver := flag.String("version", "", "采集的 TiDB 版本号")	mode := flag.String("mode", "full", "采集模式: full=全量, diff=增量")func main() {}	SystemVariables map[string]interface{} `json:"system_variables"`	ConfigDefaults  map[string]interface{} `json:"config_defaults"`	Version         string                 `json:"version"`type ParamSnapshot struct {// 可根据需要扩展字段// 包含 config 和 system variables 的默认值// ParamSnapshot 代表一个版本的全量参数快照)	"os"	"fmt"	"flag"	"encoding/json"import (