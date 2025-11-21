package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collectparams"
)

// 假设 collectFromTidbSource 已在同项目下可用
// import "../collectparams" 如需拆分包

func main() {
	repo := flag.String("repo", "../tidb", "TiDB 源码根目录")
	fromTag := flag.String("from-tag", "", "起始 tag（增量模式）")
	toTag := flag.String("to-tag", "", "目标 tag（增量模式）")
	all := flag.Bool("all", false, "全量重建模式")
	out := flag.String("out", "knowledge", "输出目录")
	flag.Parse()

	if *all {
		tags := getAllTags(*repo)
		fmt.Printf("[调试] 采集到的所有 LTS tag: %v\n", tags)
		for _, tag := range tags {
			// 适配新目录结构：knowledge/大版本/patch版本/defaults.json
			parts := strings.SplitN(tag, ".", 3)
			if len(parts) != 3 {
				fmt.Printf("tag %s 结构异常，跳过\n", tag)
				continue
			}
			major := parts[0] + "." + parts[1]
			patch := tag
			outFile := filepath.Join(*out, major, patch, "defaults.json")
			fmt.Printf("[全量] 采集 %s ...\n", tag)
			snap, err := collectparams.CollectFromTidbSource(*repo, tag)
			if err != nil {
				fmt.Printf("采集 %s 失败: %v\n", tag, err)
				continue
			}
			writeSnapshot(outFile, snap)
		}
		fmt.Println("全量采集完成")
		return
	}

	if *fromTag != "" && *toTag != "" {
		tags := getTagsInRange(*repo, *fromTag, *toTag)
		for _, tag := range tags {
			parts := strings.SplitN(tag, ".", 3)
			if len(parts) != 3 {
				fmt.Printf("tag %s 结构异常，跳过\n", tag)
				continue
			}
			major := parts[0] + "." + parts[1]
			patch := tag
			outFile := filepath.Join(*out, major, patch, "defaults.json")
			fmt.Printf("[增量] 采集 %s ...\n", tag)
			snap, err := collectparams.CollectFromTidbSource(*repo, tag)
			if err != nil {
				fmt.Printf("采集 %s 失败: %v\n", tag, err)
				continue
			}
			writeSnapshot(outFile, snap)
		}
		fmt.Println("增量采集完成")
		return
	}

	fmt.Println("请指定 --all 或 --from-tag/--to-tag")
	os.Exit(1)
}

func writeSnapshot(path string, snap interface{}) {
	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.MarshalIndent(snap, "", "  ")
	os.WriteFile(path, data, 0644)
	fmt.Printf("已写入 %s\n", path)
}

// getAllTags/ getTagsInRange 需调用 git tag --list 并排序
func getAllTags(repo string) []string {
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[调试] repo 路径转换失败: %v\n", err)
		absRepo = repo
	}
	fmt.Printf("[调试] repo dir: %s\n", absRepo)
	cmd := exec.Command("git", "tag", "--list", "--sort=version:refname")
	cmd.Dir = absRepo
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git tag failed: %v\n", err)
		return nil
	}
	fmt.Fprintf(os.Stderr, "[调试] git tag --list 原始输出:\n%s\n", string(out))
	lines := strings.Split(string(out), "\n")
	// 匹配 v6.5.1、v7.1.2、v8.5.12 等三段式 LTS patch 版本，且不包含 -
	re := regexp.MustCompile(`^v(6\.5|7\.1|7\.5|8\.1|8\.5)\.\d+$`)
	var tags []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if re.MatchString(line) && !strings.Contains(line, "-") {
			tags = append(tags, line)
		}
	}
	fmt.Fprintf(os.Stderr, "[调试] 正则过滤后 LTS tag: %v\n", tags)
	sort.Strings(tags)
	return tags
}

func getTagsInRange(repo, from, to string) []string {
	all := getAllTags(repo)
	var res []string
	found := false
	for _, tag := range all {
		if tag == from {
			found = true
		}
		if found {
			res = append(res, tag)
		}
		if tag == to {
			break
		}
	}
	return res
}

// 你可以 import "../collectparams" 并调用其采集逻辑// collectFromTidbSource 直接复用 collectparams/main.go 的实现}	return res	}		}			break		if tag == to {		}			res = append(res, tag)		if found {		}			found = true		if tag == from {	for _, tag := range all {	found := false	var res []string	all := getAllTags(repo)func getTagsInRange(repo, from, to string) []string {}	return []string{"v6.5.0", "v7.1.0", "v7.5.0", "v8.1.0", "v8.5.0"}	// TODO: 实现 git tag --list 过滤 LTSfunc getAllTags(repo string) []string {// getAllTags/ getTagsInRange 需调用 git tag --list 并排序}	os.Exit(1)	fmt.Println("请指定 --all 或 --from-tag/--to-tag")	}		return		fmt.Println("增量采集完成")		}			// TODO: 写入 outFile			}				continue				fmt.Printf("采集 %s 失败: %v\n", tag, err)			if err != nil {			_, err := collectFromTidbSource(*repo, tag)			fmt.Printf("[增量] 采集 %s ...\n", tag)			outFile := filepath.Join(*out, tag+".defaults.json")		for _, tag := range tags {		tags := getTagsInRange(*repo, *fromTag, *toTag)		// 增量采集：只采集 fromTag~toTag 范围	if *fromTag != "" && *toTag != "" {	}		return		fmt.Println("全量采集完成")		}			// TODO: 写入 outFile			}				continue				fmt.Printf("采集 %s 失败: %v\n", tag, err)			if err != nil {			_, err := collectFromTidbSource(*repo, tag)			fmt.Printf("[全量] 采集 %s ...\n", tag)			outFile := filepath.Join(*out, tag+".defaults.json")		for _, tag := range allTags {		allTags := getAllTags(*repo)		// 全量重建：遍历所有 LTS tag	if *all {	flag.Parse()	out := flag.String("out", "knowledge", "输出目录")	all := flag.Bool("all", false, "全量重建模式")	toTag := flag.String("to-tag", "", "目标 tag（增量模式）")	fromTag := flag.String("from-tag", "", "起始 tag（增量模式）")	repo := flag.String("repo", "../tidb", "TiDB 源码根目录")func main() {)	"strings"	"path/filepath"	"os"	"fmt"	"flag"import (
