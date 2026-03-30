

/*
下面是测试实例
*/

const puppeteer = require('puppeteer-core');

(async () => {
  // 连接 Browserless（注意 token）
  const browser = await puppeteer.connect({
    browserWSEndpoint: 'ws://localhost:3000?token=your-secret-token'
  });

  const page = await browser.newPage();

  // 访问百度
  await page.goto('https://www.baidu.com', {
    waitUntil: 'networkidle2',
    timeout: 30000
  });

  // 截图
  await page.screenshot({
    path: 'baidu.png',
    fullPage: true
  });

  console.log('✅ 截图完成：baidu.png');

  await browser.close();
})();
