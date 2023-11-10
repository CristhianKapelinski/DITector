package analyzer

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Musso12138/dockercrawler/analyzer/misconfiguration"
	"github.com/Musso12138/dockercrawler/myutils"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// 用于Asky与服务器通信确定任务情况
type AskYTask struct {
	Code string       `json:"code"`
	Data AskYTaskData `json:"data"`
}

type AskYTaskData struct {
	TaskID     string `json:"taskid"`
	FileName   string `json:"fileName"`
	Md5        string `json:"md5"`
	Sha1       string `json:"sha1"`
	UserID     int64  `json:"userId"`
	Status     string `json:"status"`
	Descr      string `json:"descr"`
	CreateDate string `json:"ceateDate"`
	Flag1      string `json:"flag1"`
	Flag2      string `json:"flag2"`
}

// 用于接受服务器返回的SCA和漏洞匹配结果
type AskYReport struct {
	Code string   `json:"code"`
	Data AskYData `json:"data"`
}

type AskYData struct {
	ReportData AskYReportData `json:"reportData"`
}

type AskYReportData struct {
	Component    []AskYComponent `json:"component"`
	Total        int             `json:"total"`
	ComponentNum int             `json:"componentNum"`
	VulnInfo     []AskYVulnInfo  `json:"vulnInfo"`
}

type AskYComponent struct {
	FileName    string `json:"fileName"`
	Codetype    string `json:"codetype"`
	FilePath    string `json:"filePath"`
	FileSha1    string `json:"fileSha1"`
	FileMd5     string `json:"fileMd5"`
	FileVersion string `json:"fileVersion"`
	OpenSource  string `json:"openSource"`
}

type AskYVulnInfo struct {
	CVEID           string   `json:"cveID"`
	FileName        string   `json:"fileName"`
	VulnName        string   `json:"vlunName"`
	Description     string   `json:"description"`
	Severity        string   `json:"severity"`
	VulnType        string   `json:"vuln_type"`
	ThrType         string   `json:"ThrType"`
	ModifiedTime    string   `json:"modified_time"`
	PublishedTime   string   `json:"published_time"`
	CVSSScore       string   `json:"cvssScore"`
	Solution        string   `json:"solution"`
	FilePath        string   `json:"filePath"`
	Version         string   `json:"version"`
	ProductName     string   `json:"productName"`
	VendorName      string   `json:"vendorName"`
	AffectComponent []string `json:"affectComponent"`
	AffectFile      []string `json:"affectFile"`
}

// 用于奇安信云查的文件信誉结果
type FileReputation struct {
	Sha256          string  `json:"sha256"`
	Level           int     `json:"level"`
	MalwareName     string  `json:"malware_name"`
	MalwareTypeName string  `json:"malware_type_name"`
	FileDesc        string  `json:"file_desc"`
	Describe        string  `json:"describe"`
	MaliciousFamily string  `json:"malicious_family"`
	SandboxScore    float64 `json:"sandbox_score"`
}

func (analyzer *ImageAnalyzer) analyzeContent(ci *CurrentImage, ir *myutils.ImageResult) (*myutils.ContentResult, error) {
	res := myutils.NewContentResult()
	fileWithIssues := make(map[string]bool)
	defaultExecFiles := make(map[string]struct{})
	for _, f := range ci.defaultExecFile {
		defaultExecFiles[f] = struct{}{}
	}

	// 逐层分析layer内容，写入对应LayerResult
	for _, ld := range ci.layerWithContentList {
		layerRes, fromMongo, err := analyzer.analyzeLayer(ci.layerInfoMap[ld], fileWithIssues, defaultExecFiles)
		if err != nil {
			myutils.Logger.Error("analyze layer", ci.layerInfoMap[ld].digest, "failed with:", err.Error())
			return nil, err
		}
		ir.LayerResults[ld] = layerRes

		// 把有问题的结果文件放入当前状态表
		for _, secretInfo := range layerRes.SecretLeakages {
			fileWithIssues[secretInfo.Path] = true
		}
		for _, vulnInfo := range layerRes.Vulnerabilities {
			fileWithIssues[vulnInfo.Path] = false
		}
		for _, misconfInfo := range layerRes.Misconfigurations {
			fileWithIssues[misconfInfo.Path] = false
		}
		for _, malInfo := range layerRes.MaliciousFiles {
			fileWithIssues[malInfo.Path] = false
		}

		// 新分析的结果存入数据库
		if !fromMongo {
			if myutils.GlobalDBClient.MongoFlag {
				go func(layerRes *myutils.LayerResult) {
					if e := myutils.GlobalDBClient.Mongo.UpdateLayerResult(layerRes); e != nil {
						myutils.Logger.Error("update LayerResult", layerRes.Digest, "failed with:", e.Error())
					}
				}(layerRes)
			}
		}
	}

	// 汇总各层结果，存入全局表中（当前状态）
	fileAdded := make(map[string]int)
	for i := len(ir.Layers) - 1; i >= 0; i-- {
		layerDigest := ir.Layers[i]
		// 敏感信息泄露直接加到最终结果
		res.SecretLeakages = append(res.SecretLeakages, ir.LayerResults[layerDigest].SecretLeakages...)
		// 其他问题从顶层到底层添加，存在覆盖问题
		// 软件漏洞
		for _, vulnInfo := range ir.LayerResults[layerDigest].Vulnerabilities {
			// 不存在问题（已被修复）的文件不计入
			if _, issued := fileWithIssues[vulnInfo.Path]; !issued {
				continue
			}
			// 同层同一文件问题不覆盖（一个应用文件路径对应多个漏洞）
			// 不同层同一文件问题覆盖
			if pre, ok := fileAdded[vulnInfo.Path]; ok && pre != i {
				continue
			}
			res.Vulnerabilities = append(res.Vulnerabilities, vulnInfo)
			fileAdded[vulnInfo.Path] = i
		}
		// 错误配置
		for _, misconfInfo := range ir.LayerResults[layerDigest].Misconfigurations {
			if _, issued := fileWithIssues[misconfInfo.Path]; !issued {
				continue
			}

			if pre, ok := fileAdded[misconfInfo.Path]; ok && pre != i {
				continue
			}
			res.Misconfigurations = append(res.Misconfigurations, misconfInfo)
			fileAdded[misconfInfo.Path] = i
		}
		// 恶意软件
		for _, malInfo := range ir.LayerResults[layerDigest].MaliciousFiles {
			if _, issued := fileWithIssues[malInfo.Path]; !issued {
				continue
			}

			if pre, ok := fileAdded[malInfo.Path]; ok && pre != i {
				continue
			}
			res.MaliciousFiles = append(res.MaliciousFiles, malInfo)
			fileAdded[malInfo.Path] = i
		}
	}

	return res, nil
}

// analyzeLayer TODO: traverses and analyzes files under inputted layerDir,
// and writes results directly to layerResult.
func (analyzer *ImageAnalyzer) analyzeLayer(layer *layerInfo, fileWithIssues map[string]bool, defaultExecFiles map[string]struct{}) (*myutils.LayerResult, bool, error) {
	// 数据库在线，检查是否已被分析
	if myutils.GlobalDBClient.MongoFlag {
		if lr, err := myutils.GlobalDBClient.Mongo.FindLayerResultByDigest(layer.digest); err == nil {
			return lr, true, nil
		}
	}

	var err error

	resLock := sync.Mutex{}
	res := myutils.NewLayerResult()
	res.Instruction = layer.instruction
	res.Size = layer.size
	res.Digest = layer.digest

	wg := sync.WaitGroup{}

	// SCA: 调用asky对本地层文件做SCA和漏洞匹配
	wg.Add(1)
	go func(layerRootDir, layerDir string, layerRes *myutils.LayerResult) {
		defer wg.Done()

		report, err := scaVul(layerDir, path.Join(layerRootDir, "sca.json"))
		if err != nil {
			myutils.Logger.Error("sca and matches vuln for filepath", layerDir, "failed with:", err.Error())
			return
		}

		//TODO: component加入LayerResult
		componentList := make([]*myutils.Component, 0)
		for _, comp := range report.Data.ReportData.Component {
			relPath := getRelAbsPath(layerDir, comp.FilePath)
			componentList = append(componentList, &myutils.Component{
				Filename:    comp.FileName,
				Codetype:    comp.Codetype,
				Filepath:    relPath,
				FileSha1:    comp.FileSha1,
				FileMd5:     comp.FileMd5,
				FileVersion: comp.FileVersion,
				OpenSource:  comp.OpenSource,
			})
		}

		// 形成LayerResult的漏洞列表
		vulnList := make([]*myutils.Vulnerability, 0)
		for _, vuln := range report.Data.ReportData.VulnInfo {
			relPath := getRelAbsPath(layerDir, vuln.FilePath)
			affectFile := make([]string, len(vuln.AffectFile))
			for i, p := range vuln.AffectFile {
				tmpRel := getRelAbsPath(layerDir, p)
				affectFile[i] = tmpRel
			}
			cvss, _ := strconv.ParseFloat(vuln.CVSSScore, 64)

			vulnList = append(vulnList, &myutils.Vulnerability{
				Type:        myutils.IssueType.Vulnerability,
				Name:        vuln.CVEID,
				Part:        myutils.IssuePart.Content,
				Path:        relPath,
				LayerDigest: layer.digest,

				CVEID:           vuln.CVEID,
				Filename:        vuln.FileName,
				ProductName:     vuln.ProductName,
				VendorName:      vuln.VendorName,
				Version:         vuln.Version,
				VulnType:        vuln.VulnType,
				ThrType:         vuln.ThrType,
				PublishedTime:   vuln.PublishedTime,
				Description:     vuln.Description,
				Severity:        vuln.Severity,
				CVSSScore:       cvss,
				AffectComponent: vuln.AffectComponent,
				AffectFile:      affectFile,
			})
		}

		// 上锁写入layer
		resLock.Lock()
		defer resLock.Unlock()
		layerRes.Total = report.Data.ReportData.Total
		layerRes.ComponentNum = report.Data.ReportData.ComponentNum
		layerRes.Components = componentList
		layerRes.Vulnerabilities = vulnList
	}(layer.localRootFilePath, layer.localFilePath, res)

	// 遍历layer目录，发现需要扫描隐私泄露/错误配置/恶意软件的文件，并进行相应扫描
	if err = filepath.Walk(layer.localFilePath, analyzer.scanLayerFunc(layer, fileWithIssues, defaultExecFiles, res, &resLock)); err != nil {
		fmt.Println()
	}

	wg.Wait()
	return res, false, err
}

// scaVul TODO: 对层文件进行SCA并进行漏洞匹配
func scaVul(layerDir, dest string) (*AskYReport, error) {
	// 调用asky脚本本地SCA
	cmd := exec.Command("bash", myutils.GlobalConfig.AskyConfig.AskyFile, "-s", layerDir, "-o", dest)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	// 上传asky sca结果到天蚕API，建立检测任务
	task, err := postCreateAskYTask(client, dest)
	if err != nil {
		myutils.Logger.Error("post SCA result file", dest, "to asky server failed with:", err.Error())
		return nil, err
	}

	// 获取检测报告
	report, err := checkGetAskYReport(client, task)
	if err != nil {
		myutils.Logger.Error("check and get asky report of task", task.Data.TaskID, "failed with:", err.Error())
		return nil, err
	}

	return report, nil
}

// postCreateAskYTask 将指定SCA结果文件上传到asky服务端，返回检测任务信息
func postCreateAskYTask(client *http.Client, dest string) (*AskYTask, error) {
	file, err := os.Open(dest)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("http://beta.tqs.qianxin-inc.cn/asky/skily/uploadLog?token=%s", myutils.GlobalConfig.AskyConfig.AskyToken), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	//scaFile, err := os.Open(dest)
	//if err != nil {
	//	return nil, err
	//}
	//defer scaFile.Close()
	//
	//postReq, err := http.NewRequest(http.MethodPost,
	//	fmt.Sprintf("http://beta.tqs.qianxin-inc.cn/asky/skily/uploadLog?token=%s", myutils.GlobalConfig.AskyConfig.AskyToken),
	//	scaFile,
	//)
	//if err != nil {
	//	return nil, err
	//}
	//
	//resp, err := client.Do(postReq)
	//if err != nil {
	//	return nil, err
	//}
	//defer resp.Body.Close()

	//body, err := io.ReadAll(resp.Body)
	//test := string(body)
	//fmt.Println(test)
	//if err != nil {
	//	return nil, err
	//}

	task := new(AskYTask)
	if err = json.Unmarshal(respBody, task); err != nil {
		return nil, err
	}
	if task.Data.Status != "0" && task.Data.Status != "1" && task.Data.Status != "2" {
		return nil, fmt.Errorf("asky start with task code %s", task.Code)
	}

	return task, nil
}

// checkGetAskYReport 查询服务端检测状态，检测完成后获取检测报告
func checkGetAskYReport(client *http.Client, task *AskYTask) (*AskYReport, error) {
	// 等待asky服务端检测任务完成
	failCnt := 0
	for {
		time.Sleep(1 * time.Second)

		// 获取检测状态
		statusReq, err := http.NewRequest(http.MethodGet,
			fmt.Sprintf("http://beta.tqs.qianxin-inc.cn/asky/skily/queryTaskStatus?sha1=%s&tid=%s", task.Data.Sha1, task.Data.Flag2),
			nil,
		)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(statusReq)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		state := new(AskYTask)
		if err := json.Unmarshal(body, state); err != nil {
			return nil, err
		}

		// 检测完成时退出循环
		if state.Data.Status == "2" {
			break
		} else if state.Data.Status == "3" || state.Data.Status == "4" {
			return nil, fmt.Errorf("asky server response exception (task status %s)", state.Data.Status)
		}

		// 最多等待一个任务五分钟
		failCnt++
		if failCnt > 300 {
			return nil, fmt.Errorf("waiting for asky server scanning filepath %s timeout after %d retries", task.Data.FileName, failCnt)
		}
	}

	// 获取检测结果
	reportReq, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("http://beta.tqs.qianxin-inc.cn/asky/skily/queryReport/%s?token=%s", task.Data.TaskID, myutils.GlobalConfig.AskyConfig.AskyToken),
		nil)
	if err != nil {
		return nil, err
	}

	reportResp, err := client.Do(reportReq)
	if err != nil {
		return nil, err
	}
	defer reportResp.Body.Close()

	body, err := io.ReadAll(reportResp.Body)
	if err != nil {
		return nil, err
	}

	report := new(AskYReport)
	if err := json.Unmarshal(body, report); err != nil {
		return nil, err
	}

	return report, nil
}

// scanLayerFunc 返回一个用于遍历layer目录时扫描文件内容的函数
func (analyzer *ImageAnalyzer) scanLayerFunc(layer *layerInfo, fileWithIssues map[string]bool, defaultExecFiles map[string]struct{}, layerRes *myutils.LayerResult, layerResMu *sync.Mutex) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			myutils.Logger.Error("scan layer file", layer.localFilePath, "failed with:", err.Error())
			return err
		}

		// 跳过文件夹
		if info.IsDir() {
			return nil
		}

		relPath := getRelAbsPath(layer.localFilePath, path)

		// 基于当前状态删除过往扫描记录
		if secretFlag, ok := fileWithIssues[relPath]; ok && !secretFlag {
			delete(fileWithIssues, relPath)
		}

		// 根据文件路径确定扫描内容
		// 扫描隐私泄露
		if FileNeedScanSecrets(relPath) {
			secrets, err := analyzer.scanSecretsInFile(path)
			if err != nil {
				myutils.Logger.Error("scan secret leakages for file", path, "failed with:", err.Error())
				return err
			}
			for _, secret := range secrets {
				secret.Part = myutils.IssuePart.Content
				secret.Path = relPath
				secret.LayerDigest = layer.digest

				layerResMu.Lock()
				layerRes.SecretLeakages = append(layerRes.SecretLeakages, secret)
				layerResMu.Unlock()
			}
		}

		// 配置文件，检测错误配置
		if need, app := misconfiguration.FileNeedScan(relPath); need {
			misConfs, err := misconfiguration.ScanFileMisconfiguration(path, app)
			if err != nil {
				myutils.Logger.Error("scan misconfiguration of app", app, "for file", path, "failed with:", err.Error())
				return err
			}
			for _, misConf := range misConfs {
				misConf.Part = myutils.IssuePart.Content
				misConf.Path = relPath
				misConf.LayerDigest = layer.digest

				layerResMu.Lock()
				layerRes.Misconfigurations = append(layerRes.Misconfigurations, misConf)
				layerResMu.Unlock()
			}
		}

		// 默认执行路径文件，检测恶意性
		// Entry File，检测恶意性
		if _, ok := defaultExecFiles[relPath]; ok {
			malFile, malFlag, err := scanFileMalicious(path)
			if err != nil {
				return err
			}
			if malFlag {
				malFile.Part = myutils.IssuePart.Content
				malFile.Path = relPath
				malFile.LayerDigest = layer.digest

				layerResMu.Lock()
				layerRes.MaliciousFiles = append(layerRes.MaliciousFiles, malFile)
				layerResMu.Unlock()
			}
		}

		return nil
	}
}

// scanFileMalicious 利用奇安信云查接口检查文件是否恶意
func scanFileMalicious(filepath string) (*myutils.MaliciousFile, bool, error) {
	reputation, err := getFileReputation(filepath)
	if err != nil {
		return nil, false, err
	}

	if reputation.MalwareName == "" {
		return nil, false, nil
	}

	i := &myutils.MaliciousFile{
		Type:        myutils.IssueType.MaliciousFile,
		Name:        reputation.MalwareName,
		Description: reputation.Describe,
		Severity:    "HIGH",

		Sha256:          reputation.Sha256,
		Level:           reputation.Level,
		MalwareTypeName: reputation.MalwareTypeName,
		FileDesc:        reputation.FileDesc,
		Describe:        reputation.Describe,
		MaliciousFamily: reputation.MaliciousFamily,
		SandboxScore:    reputation.SandboxScore,
	}

	return i, true, nil
}

func getFileReputation(filepath string) (*FileReputation, error) {
	h, err := myutils.Sha256File(filepath)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("http://tqs.qianxin-inc.cn/file/v1/files/%s/reputation", h),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := new(FileReputation)
	err = json.Unmarshal(body, res)

	return res, err
}

func getRelAbsPath(layerDir, path string) string {
	relPath, _ := filepath.Rel(layerDir, path)
	return "/" + relPath
}
