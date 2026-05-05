package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lengzhao/clawbridge/client"
	_ "github.com/lengzhao/clawbridge/drivers" // register onboarding flows for ListOnboardingDrivers / Describe

	"golang.org/x/term"
)

// RunChannelDriverPick lists registered clawbridge onboarding drivers (same discovery as channel onboard).
// Returns skip=true for choice 0 / empty, stdin non-terminal, or when no drivers registered.
func RunChannelDriverPick(out io.Writer, in io.Reader) (driver string, skip bool, err error) {
	drivers := client.ListOnboardingDrivers()
	if len(drivers) == 0 {
		fmt.Fprintln(out, "未注册任何渠道 onboarding（请确认二进制已链接 clawbridge/drivers）。")
		fmt.Fprintln(out, "稍后可运行：oneclaw channel list-drivers")
		return "", true, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(out, "stdin 非终端，跳过渠道步骤。")
		return "", true, nil
	}

	fmt.Fprintln(out, "选择渠道驱动（clawbridge；序号 0 跳过）：")
	fmt.Fprintln(out, "  0) 跳过")
	for i, d := range drivers {
		label := channelDriverLabel(d)
		fmt.Fprintf(out, "  %d) %s\n", i+1, label)
	}
	fmt.Fprint(out, "请输入序号并回车 [0]: ")
	line, err := readLineChannel(in)
	if err != nil {
		return "", false, err
	}
	choice := strings.TrimSpace(line)
	if choice == "" || choice == "0" {
		fmt.Fprintln(out, "已跳过渠道配置；稍后可运行：oneclaw channel onboard <driver>")
		return "", true, nil
	}
	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(drivers) {
		return "", false, fmt.Errorf("onboard: invalid channel choice %q", choice)
	}
	return drivers[idx-1], false, nil
}

// RunChannelContinuePrompt asks whether to onboard another driver (y/N). Non-terminal stdin returns false.
func RunChannelContinuePrompt(out io.Writer, in io.Reader) (yes bool, err error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, nil
	}
	fmt.Fprint(out, "是否继续添加其他渠道？[y/N]: ")
	line, err := readLineChannel(in)
	if err != nil {
		return false, err
	}
	s := strings.ToLower(strings.TrimSpace(line))
	return s == "y" || s == "yes", nil
}

func channelDriverLabel(driver string) string {
	desc, err := client.DescribeOnboardingDriver(driver)
	if err != nil || strings.TrimSpace(desc.DisplayName) == "" {
		return driver
	}
	if desc.DisplayName == driver {
		return driver
	}
	return fmt.Sprintf("%s（%s）", desc.DisplayName, driver)
}

func readLineChannel(in io.Reader) (string, error) {
	br := bufio.NewReader(in)
	s, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(strings.TrimSuffix(s, "\n")), nil
}
