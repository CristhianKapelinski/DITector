package analyzer

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Musso12138/dockercrawler/myutils"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
	UserID     string `json:"userId"`
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
	CVSSScore       float64  `json:"cvssScore"`
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

	//// 遍历各层结果，存入全局表中（当前状态）
	//for _, ld := range ir.Layers {
	//	for filepath, fileIs := range ir.LayerResults[ld].FileIssues {
	//		// 如果下层中扫过filepath，将其中的隐私信息泄露问题加进来
	//		tmpIs := make([]*myutils.Issue, len(fileIs))
	//		copy(tmpIs, fileIs)
	//		if preIs, ok := ir.FileIssues[filepath]; ok {
	//			for _, preI := range preIs {
	//				if preI.Type == myutils.IssueType.SecretLeakage {
	//					myutils.AddIssue(tmpIs, preI)
	//				}
	//			}
	//		}
	//
	//		ir.FileIssues[filepath] = tmpIs
	//	}
	//}
	//
	//// 汇总各层file，形成最终结果
	//for _, fileIs := range ir.FileIssues {
	//	myutils.AddIssue(res, fileIs...)
	//}

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
	go func(layerDir string, layerRes *myutils.LayerResult) {
		defer wg.Done()

		report, err := scaVul(layerDir, path.Join(layerDir, "..", "sca.json"))
		if err != nil {
			myutils.Logger.Error("sca and matches vuln for filepath", layerDir, "failed with:", err.Error())
			return
		}

		//TODO: component加入LayerResult

		// 形成LayerResult的漏洞列表
		vulnList := make([]myutils.Vulnerability, 0)
		for _, vuln := range report.Data.ReportData.VulnInfo {
			relPath, _ := filepath.Rel(layerDir, vuln.FilePath)
			affectFile := make([]string, len(vuln.AffectFile))
			for i, p := range vuln.AffectFile {
				tmpRel, _ := filepath.Rel(layerDir, p)
				affectFile[i] = tmpRel
			}

			vulnList = append(vulnList, myutils.Vulnerability{
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
				CVSSScore:       vuln.CVSSScore,
				AffectComponent: vuln.AffectComponent,
				AffectFile:      affectFile,
			})
		}

		// 上锁写入layer
		resLock.Lock()
		defer resLock.Unlock()
		layerRes.Total = report.Data.ReportData.Total
		layerRes.ComponentNum = report.Data.ReportData.ComponentNum
		layerRes.Vulnerabilities = vulnList
	}(layer.localFilePath, res)

	// 遍历layer目录，发现需要扫描隐私泄露/错误配置/恶意软件的文件，并进行相应扫描
	if err = filepath.Walk(layer.localFilePath, scanLayerFunc(layer, fileWithIssues, defaultExecFiles, res, &resLock)); err != nil {
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

	scaFile, err := os.Open(dest)
	if err != nil {
		return nil, err
	}
	defer scaFile.Close()

	// 上传asky sca结果到天蚕API
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	postReq, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://beta.tqs.qianxin-inc.cn/asky/skily/uploadLog?token=%s", myutils.GlobalConfig.AskyConfig.AskyToken),
		scaFile,
	)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(postReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	task := new(AskYTask)
	if err = json.Unmarshal(body, task); err != nil {
		return nil, err
	}
	if task.Data.Status != "0" && task.Data.Status != "1" && task.Data.Status != "2" {
		return nil, fmt.Errorf("asky start with task code %s", task.Code)
	}

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
			return nil, fmt.Errorf("waiting for asky server scanning filepath %s timeout after %d retries", dest, failCnt)
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

	body, err = io.ReadAll(reportResp.Body)
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
func scanLayerFunc(layer *layerInfo, fileWithIssues map[string]bool, defaultExecFiles map[string]struct{}, layerRes *myutils.LayerResult, layerResMu *sync.Mutex) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			myutils.Logger.Error("scan layer file", layer.localFilePath, "failed with:", err.Error())
			return err
		}

		// 跳过文件夹
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(layer.localFilePath, path)
		if err != nil {
			return err
		}
		// 基于当前状态删除过往扫描记录
		if secretFlag, ok := fileWithIssues[relPath]; ok && !secretFlag {
			delete(fileWithIssues, relPath)
		}

		// TODO: 根据文件路径确定扫描内容

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
		Part:        myutils.IssuePart.Content,
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

	req, err := http.NewRequest("GET",
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
