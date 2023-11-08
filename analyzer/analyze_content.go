package analyzer

import (
	"encoding/json"
	"fmt"
	"github.com/Musso12138/dockercrawler/myutils"
	"io"
	"net/http"
	"sync"
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
	Data AskYData `json:"data"`
}

type AskYData struct {
	ReportData AskYReportData `json:"reportData"`
}

type AskYReportData struct {
	Total        int            `json:"total"`
	ComponentNum int            `json:"componentNum"`
	VulnInfo     []AskYVulnInfo `json:"vulnInfo"`
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
	wg := sync.WaitGroup{}

	// 逐层分析layer内容，写入对应LayerResult
	for _, ld := range ci.layerWithContentList {
		wg.Add(1)
		go func(layer *layerInfo) {
			defer wg.Done()
			layerRes, fromMongo, err := analyzer.analyzeLayer(layer)
			if err != nil {
				myutils.Logger.Error("analyze layer", layer.digest, "failed with:", err.Error())
				return
			}
			ir.LayerResults[layer.digest] = layerRes

			if !fromMongo {
				if myutils.GlobalDBClient.MongoFlag {
					go func(layerRes *myutils.LayerResult) {
						if e := myutils.GlobalDBClient.Mongo.UpdateLayerResult(layerRes); e != nil {
							myutils.Logger.Error("update LayerResult", layerRes.Digest, "failed with:", e.Error())
						}
					}(layerRes)
				}
			}
		}(ci.layerInfoMap[ld])
	}

	// 等待各层分析结束
	wg.Wait()

	// 遍历各层结果，存入全局表中（当前状态）
	for _, ld := range ir.Layers {
		for filepath, fileIs := range ir.LayerResults[ld].FileIssues {
			// 如果下层中扫过filepath，将其中的隐私信息泄露问题加进来
			tmpIs := make([]*myutils.Issue, len(fileIs))
			copy(tmpIs, fileIs)
			if preIs, ok := ir.FileIssues[filepath]; ok {
				for _, preI := range preIs {
					if preI.Type == myutils.IssueType.SecretLeakage {
						myutils.AddIssue(tmpIs, preI)
					}
				}
			}

			ir.FileIssues[filepath] = tmpIs
		}
	}

	// 汇总各层file，形成最终结果
	for _, fileIs := range ir.FileIssues {
		myutils.AddIssue(res, fileIs...)
	}

	return res, nil
}

// analyzeLayer TODO: traverses and analyzes files under inputted layerDir,
// and writes results directly to layerResult.
func (analyzer *ImageAnalyzer) analyzeLayer(layer *layerInfo) (*myutils.LayerResult, bool, error) {
	// 数据库在线，检查是否已被分析
	if myutils.GlobalDBClient.MongoFlag {
		if lr, err := myutils.GlobalDBClient.Mongo.FindLayerResultByDigest(layer.digest); err == nil {
			return lr, true, nil
		}
	}

	resLock := sync.Mutex{}
	res := myutils.NewLayerResult()
	res.Instruction = layer.instruction
	res.Size = layer.size
	res.Digest = layer.digest

	wg := sync.WaitGroup{}

	// SCA: 调用asky对本地层文件做
	wg.Add(1)
	go func(layerDir string, layerRes *myutils.LayerResult) {
		defer wg.Done()

		report, err := scaVul(layerDir)
		if err != nil {
			myutils.Logger.Error("sca and matches vuln for filepath failed with:", err.Error())
			return
		}

		resLock.Lock()
		defer resLock.Unlock()
		layerRes.Total = report.Data.ReportData.Total
		layerRes.ComponentNum = report.Data.ReportData.ComponentNum
		for _, vuln := range report.Data.ReportData.VulnInfo {

		}

	}(layer.localFilePath, res)

	wg.Wait()
	return res, false, nil
}

// scaVul TODO: 对层文件进行SCA并进行漏洞匹配
func scaVul(layerDir string) (*AskYReport, error) {
	// 调用asky脚本本地SCA

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

	resp, err := http.Get(fmt.Sprintf("https://tqs.qianxin-inc.cn/file/v1/files/%s/reputation", h))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := new(FileReputation)
	err = json.Unmarshal(body, res)

	return res, err
}
