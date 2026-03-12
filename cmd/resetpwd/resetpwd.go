package resetpwd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/engigu/baihu-panel/internal/bootstrap"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/utils"
)

func Run(args []string) {
	// 基础环境初始化
	bootstrap.InitBasic()
	settingsService := services.NewSettingsService()
	if err := settingsService.InitSettings(); err != nil {
		fmt.Printf("初始化系统设置失败: %v\n", err)
		return
	}
	userService := services.NewUserService()

	var username string

	if len(args) >= 1 {
		username = args[0]
	} else {
		username = "admin"
	}

	fmt.Printf("此操作将重置用户 [%s] 的密码，是否继续? (y/N): ", username)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		fmt.Println("操作已取消。")
		return
	}

	fmt.Printf("请输入用户 [%s] 的新密码 (留空则自动随机生成): ", username)
	inputPwd, _ := reader.ReadString('\n')
	newPassword := strings.TrimSpace(inputPwd)
	if newPassword == "" {
		newPassword = utils.RandomString(12)
		fmt.Println("未输入密码，系统已自动生成。")
	}

	user := userService.GetUserByUsername(username)
	if user == nil {
		fmt.Printf("找不到用户 [%s]\n", username)
		return
	}

	err := userService.UpdatePassword(user.ID, newPassword)
	if err != nil {
		fmt.Printf("重置密码失败: %v\n", err)
		return
	}

	fmt.Println("--------------------------------------------------")
	fmt.Printf("用户 [%s] 密码已重置成功:\n", username)
	fmt.Printf("新密码: %s\n", newPassword)
	fmt.Println("请妥善保管您的新密码，并登录后及时修改。")
	fmt.Println("--------------------------------------------------")
}
