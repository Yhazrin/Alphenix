"use client";

import { useState } from "react";
import { Server, Globe, Lock, Play, Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { useWorkspaceStore } from "@/features/workspace";

export function ServerTab() {
  const workspace = useWorkspaceStore((s) => s.workspace);
  const [serverUrl, setServerUrl] = useState(
    process.env.NEXT_PUBLIC_MULTICODE_SERVER_URL || "http://localhost:8080"
  );
  const [useTunnel, setUseTunnel] = useState(false);
  const [tunnelUrl, setTunnelUrl] = useState("");
  const [copied, setCopied] = useState(false);

  const handleCopyCommand = (command: string) => {
    navigator.clipboard.writeText(command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const localCommands = {
    connect: `export MULTICODE_SERVER_URL="${serverUrl}"
multicode daemon start`,
    deploy: `cd /path/to/multicode
export DATABASE_URL="postgres://user:pass@host:5432/multicode"
./multicode-server`,
  };

  return (
    <div className="space-y-8">
      {/* Server Connection */}
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Server className="h-5 w-5 text-purple-500" />
          <h2 className="text-sm font-semibold">服务器连接</h2>
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="server-url">服务器地址</Label>
            <div className="flex gap-2">
              <Input
                id="server-url"
                value={serverUrl}
                onChange={(e) => setServerUrl(e.target.value)}
                placeholder="http://localhost:8080"
              />
              <Button variant="outline">测试连接</Button>
            </div>
            <p className="text-xs text-muted-foreground">
              填写你的 multicode 服务器地址。如果是本地开发，默认为 localhost:8080
            </p>
          </div>
        </div>
      </section>

      {/* Tunnel Configuration */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Globe className="h-5 w-5 text-blue-500" />
            <h2 className="text-sm font-semibold">内网穿透配置</h2>
          </div>
          <Switch
            checked={useTunnel}
            onCheckedChange={setUseTunnel}
          />
        </div>

        {useTunnel && (
          <div className="space-y-4 p-4 bg-blue-50 dark:bg-blue-950/20 rounded-lg">
            <p className="text-sm text-muted-foreground">
              开启内网穿透后，你可以从任何设备访问你的 multicode 服务器。
              推荐使用 ngrok 或 Cloudflare Tunnel。
            </p>

            <div className="space-y-2">
              <Label htmlFor="tunnel-url">穿透后的公网地址</Label>
              <Input
                id="tunnel-url"
                value={tunnelUrl}
                onChange={(e) => setTunnelUrl(e.target.value)}
                placeholder="https://xxxx.ngrok.io"
              />
            </div>

            <div className="space-y-2">
              <Label>快速启动 ngrok</Label>
              <div className="bg-black/5 dark:bg-white/5 p-3 rounded-lg font-mono text-xs space-y-1">
                <p># 安装 ngrok (macOS)</p>
                <p>brew install ngrok</p>
                <p className="pt-2"># 启动隧道</p>
                <p>ngrok http 8080</p>
              </div>
            </div>

            {tunnelUrl && (
              <div className="space-y-2">
                <Label>客户端连接命令</Label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 bg-black/5 dark:bg-white/5 p-2 rounded text-xs overflow-x-auto">
                    export MULTICODE_SERVER_URL="{tunnelUrl}"
                    <br />
                    multicode daemon start
                  </code>
                  <Button
                    size="icon"
                    variant="outline"
                    onClick={() =>
                      handleCopyCommand(
                        `export MULTICODE_SERVER_URL="${tunnelUrl}"\nmulticode daemon start`
                      )
                    }
                  >
                    {copied ? (
                      <Check className="h-4 w-4 text-green-500" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </section>

      {/* Deployment Guide */}
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <Play className="h-5 w-5 text-green-500" />
          <h2 className="text-sm font-semibold">部署指南</h2>
        </div>

        <div className="space-y-4">
          {/* Server Deployment */}
          <div className="border rounded-lg p-4 space-y-3">
            <h3 className="text-sm font-medium flex items-center gap-2">
              <Server className="h-4 w-4" />
              1. 部署服务器
            </h3>
            <div className="bg-black/5 dark:bg-white/5 p-3 rounded-lg font-mono text-xs space-y-1">
              <p># 编译 server</p>
              <p>cd server && go build -o ../multicode-server ./cmd/server</p>
              <p className="pt-2"># 启动服务</p>
              <p>export PORT=8080</p>
              <p>export DATABASE_URL="postgres://..."</p>
              <p>./multicode-server</p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleCopyCommand(localCommands.deploy)}
            >
              <Copy className="h-4 w-4 mr-2" />
              复制命令
            </Button>
          </div>

          {/* Client Connection */}
          <div className="border rounded-lg p-4 space-y-3">
            <h3 className="text-sm font-medium flex items-center gap-2">
              <Globe className="h-4 w-4" />
              2. 连接客户端
            </h3>
            <p className="text-xs text-muted-foreground">
              在其他设备上运行以下命令连接到服务器：
            </p>
            <div className="bg-black/5 dark:bg-white/5 p-3 rounded-lg font-mono text-xs space-y-1">
              <p>export MULTICODE_SERVER_URL="{serverUrl}"</p>
              <p>multicode login</p>
              <p>multicode daemon start</p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleCopyCommand(localCommands.connect)}
            >
              <Copy className="h-4 w-4 mr-2" />
              复制命令
            </Button>
          </div>

          {/* Security Note */}
          <div className="border border-amber-200 dark:border-amber-900 rounded-lg p-4 bg-amber-50 dark:bg-amber-950/20">
            <div className="flex items-center gap-2 text-amber-700 dark:text-amber-400">
              <Lock className="h-4 w-4" />
              <span className="text-sm font-medium">安全建议</span>
            </div>
            <ul className="mt-2 text-xs text-amber-600 dark:text-amber-500 space-y-1">
              <li>• 生产环境建议使用 HTTPS</li>
              <li>• 使用 Nginx 或 Caddy 配置 SSL 反向代理</li>
              <li>• 设置防火墙规则限制访问</li>
              <li>• 定期更新 API Token</li>
            </ul>
          </div>
        </div>
      </section>

      {/* Workspace Info */}
      {workspace && (
        <section className="space-y-4">
          <h2 className="text-sm font-semibold">当前工作区</h2>
          <div className="bg-muted/50 rounded-lg p-4 space-y-2">
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">名称</span>
              <span className="font-medium">{workspace.name}</span>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">ID</span>
              <code className="text-xs bg-muted px-2 py-0.5 rounded">
                {workspace.id}
              </code>
            </div>
          </div>
        </section>
      )}
    </div>
  );
}
