#!/bin/bash

# 面向终端的交互式 Hook 拦截器
# 目的是为了在终端执行 node 相关任务时具有与 Go 系统底层相同的 NODE_PATH 环境
#
# 支持以下各种执行场景：
# 1. mise exec node@23.11.1 -- node index.js      (指明具体版本)
# 2. mise exec python@3 node@23 -- node index.js  (混合语言注入)
# 3. mise exec node -- node index.js              (显式指定环境但使用默认版本)
# 4. mise exec -- node index.js                   (完全隐式环境，由项目配置决定)
mise() {
  if [[ "$1" == "exec" || "$1" == "x" ]]; then
    local node_spec=""
    # 优先从命令行参数中提取指定的 node 版本（如 node@23...）
    for arg in "$@"; do
      if [[ "$arg" == "--" ]]; then
        break
      elif [[ "$arg" == "node" || "$arg" == node@* ]]; then
        node_spec="$arg"
        break
      fi
    done

    # 如果命令行参数中没写，则尝试检测该环境下是否有已激活的默认 node
    if [[ -z "$node_spec" ]]; then
      if command mise which node >/dev/null 2>&1; then
        node_spec="node"
      fi
    fi

    if [[ -n "$node_spec" ]]; then
      local node_dir=$(command mise where "$node_spec" 2>/dev/null)
      if [[ -n "$node_dir" ]]; then
        NODE_PATH="$node_dir/lib/node_modules" command mise "$@"
        return $?
      fi
    fi
  fi
  # 非 Node 场景或拦截失败，正常向下执行
  command mise "$@"
}

# 导出此函数，供所有子环境继承使用
export -f mise
