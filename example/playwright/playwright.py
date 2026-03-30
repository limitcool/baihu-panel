"""
Python 示例：通过 Playwright 连接 Browserless，并抓取页面截图。
运行前请先在 Baihu 的“语言依赖”中为 Python 安装 playwright 包。
如果使用 Browserless，请不要执行 `playwright install`。
"""

from playwright.sync_api import sync_playwright


def main() -> None:
    with sync_playwright() as p:
        browser = p.chromium.connect_over_cdp(
            "http://browser:3000?token=your-secret-token"
        )

        page = browser.new_page()
        page.goto("https://www.baidu.com", wait_until="networkidle", timeout=30000)
        page.screenshot(path="baidu.png", full_page=True)

        print("截图已保存为 baidu.png")
        browser.close()


if __name__ == "__main__":
    main()
