package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	defer os.Remove("captcha.png")
	var (
		loginUrl = ""
		username = ""
		password = ""
		headless bool
	)
	flag.StringVar(&loginUrl, "loginUrl", loginUrl, "登录页面URL")
	flag.StringVar(&username, "username", username, "用户名")
	flag.StringVar(&password, "password", password, "密码")
	flag.BoolVar(&headless, "headless", false, "是否开启无头模式")
	flag.Parse()
	go func() {
		var pyBin string
		_, err1 := exec.LookPath("python")
		if err1 == nil {
			pyBin = "python"
		} else {
			_, err2 := exec.LookPath("python3")
			if err2 != nil {
				log.Fatalf("未找到python3或python，请先安装好全部前置环境")
			}
			pyBin = "python3"
		}
		finalCmd := fmt.Sprintf("%s ddddocr_server/app/main.py", pyBin)
		goos := runtime.GOOS
		if goos == "windows" {
			exec.Command("cmd", "/c", finalCmd).CombinedOutput()
		} else {
			exec.Command("bash", "-c", finalCmd).CombinedOutput()
		}
	}()
	time.Sleep(time.Microsecond * 500)
	FormLogin(loginUrl, username, password, headless)
}

// 传入登录地址，用户名和密码，自动填入登录表单
// 自动检测识别验证码并填写
// 自动刷新验证码
// 自动登录
func FormLogin(loginUrl, username, password string, headless bool) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("ignore-certificate-errors", true),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, time.Hour)
	defer cancel()
	if err := chromedp.Run(ctx,
		chromedp.Navigate(loginUrl),
		chromedp.Sleep(time.Second*2),
		chromedp.WaitVisible(`form`, chromedp.ByQuery),
	); err != nil {
		log.Fatalf("导航到登录页面失败: %v", err)
	}

	var captchaImgSrc, captchaInputSelector string
	var hasCaptcha bool

	var captchaResult struct {
		CaptchaInputSelector string `json:"captchaInputSelector"`
		CaptchaImgSrc        string `json:"captchaImgSrc"`
		HasCaptcha           bool   `json:"hasCaptcha"`
	}
	if err := chromedp.Run(ctx,
		chromedp.EvaluateAsDevTools(`
        (function() {
            const inputs = Array.from(document.querySelector('form')?.elements || [])
                .filter(el => el.tagName === 'INPUT' && el.type !== 'hidden');

            if (inputs.length >= 2 && 
                inputs[0].type === 'text' && 
                inputs[1].type === 'password') {
                
                let imgElement = null;
                const form = document.querySelector('form');

                if (inputs.length >= 3) {
                    if (inputs[2].nextElementSibling?.tagName === 'IMG') {
                        imgElement = inputs[2].nextElementSibling;
                    }
                    else if (form) {
                        const imgs = form.querySelectorAll('img');
                        if (imgs.length > 0) {
                            imgElement = imgs[imgs.length - 1];
                        }
                    }
                }

                if (imgElement) {
                    return {
                        captchaInputSelector: inputs[2].id ? '#' + inputs[2].id :
                            inputs[2].name ? '[name="' + inputs[2].name + '"]' :
                            'input:nth-of-type(3)',
                        captchaImgSrc: imgElement.src || '',
                        hasCaptcha: true
                    };
                }
            }
            return { hasCaptcha: false };
        })()
    `, &captchaResult),
	); err != nil {
		log.Fatalf("检查表单结构失败: %v", err)
	}
	captchaInputSelector = captchaResult.CaptchaInputSelector
	captchaImgSrc = captchaResult.CaptchaImgSrc
	hasCaptcha = captchaResult.HasCaptcha

	if hasCaptcha {
		fmt.Printf("检测到验证码，输入框选择器: %s，图片src: %s\n", captchaInputSelector, captchaImgSrc)
		var captchaBuf []byte
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(fmt.Sprintf(`img[src="%s"]`, captchaImgSrc), chromedp.ByQuery),
			chromedp.Screenshot(fmt.Sprintf(`img[src="%s"]`, captchaImgSrc), &captchaBuf, chromedp.ByQuery),
		); err != nil {
			log.Fatalf("获取验证码图片失败: %v", err)
		}
		captchaPath := "captcha.png"
		if err := ioutil.WriteFile(captchaPath, captchaBuf, 0644); err != nil {
			log.Fatalf("保存验证码图片失败: %v", err)
		}
		fmt.Printf("验证码已保存到: %s\n", captchaPath)
		ocrResp, err := CallWithFile(captchaPath, "http://localhost:28888/ocr")
		if err != nil {
			log.Fatalf("调用OCR API失败: %v\n", err)
		}
		if ocrResp.Code == 200 {
			fmt.Printf("OCR识别结果: %s\n", ocrResp.Data)
		} else {
			log.Fatalf("调用OCR API失败: %v\n", ocrResp.Message)
		}
		if err := chromedp.Run(ctx,
			chromedp.SendKeys(captchaInputSelector, ocrResp.Data, chromedp.ByQuery),
		); err != nil {
			log.Fatalf("填写验证码失败: %v\n", err)
		}
	}

	if err := chromedp.Run(ctx,
		chromedp.Sleep(time.Second),
		chromedp.SendKeys(`input[type="text"]`, username, chromedp.ByQuery),
		chromedp.SendKeys(`input[type="password"]`, password, chromedp.ByQuery),
		chromedp.Sleep(time.Second),
		chromedp.Click(`button,input[type="submit"]`, chromedp.ByQuery),
		chromedp.Sleep(time.Second*2),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(time.Second),
	); err != nil {
		log.Fatalf("登录失败: %v", err)
	}
	fmt.Println("登录成功!")
}
