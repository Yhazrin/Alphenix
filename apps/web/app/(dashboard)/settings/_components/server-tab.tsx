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
          <Server className="h-5 w-5 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Server Connection</h2>
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="server-url">Server URL</Label>
            <div className="flex gap-2">
              <Input
                id="server-url"
                value={serverUrl}
                onChange={(e) => setServerUrl(e.target.value)}
                placeholder="http://localhost:8080"
              />
              <Button variant="outline">Test Connection</Button>
            </div>
            <p className="text-xs text-muted-foreground">
              Enter your Multicode server address. For local development, the
              default is localhost:8080.
            </p>
          </div>
        </div>
      </section>

      {/* Tunnel Configuration */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Globe className="h-5 w-5 text-muted-foreground" />
            <h2 className="text-sm font-semibold">Tunnel Configuration</h2>
          </div>
          <Switch
            checked={useTunnel}
            onCheckedChange={setUseTunnel}
          />
        </div>

        {useTunnel && (
          <div className="space-y-4 rounded-lg border bg-muted/30 p-4">
            <p className="text-sm text-muted-foreground">
              Enable a tunnel to access your Multicode server from any device.
              ngrok or Cloudflare Tunnel are recommended.
            </p>

            <div className="space-y-2">
              <Label htmlFor="tunnel-url">Public tunnel URL</Label>
              <Input
                id="tunnel-url"
                value={tunnelUrl}
                onChange={(e) => setTunnelUrl(e.target.value)}
                placeholder="https://xxxx.ngrok.io"
              />
            </div>

            <div className="space-y-2">
              <Label>Quick start with ngrok</Label>
              <div className="bg-muted/50 p-3 rounded-lg font-mono text-xs space-y-1">
                <p className="text-muted-foreground"># Install ngrok (macOS)</p>
                <p>brew install ngrok</p>
                <p className="pt-2 text-muted-foreground"># Start tunnel</p>
                <p>ngrok http 8080</p>
              </div>
            </div>

            {tunnelUrl && (
              <div className="space-y-2">
                <Label>Client connection command</Label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 bg-muted/50 p-2 rounded text-xs overflow-x-auto">
                    export MULTICODE_SERVER_URL=&quot;{tunnelUrl}&quot;
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
                      <Check className="h-4 w-4 text-success" />
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
          <Play className="h-5 w-5 text-muted-foreground" />
          <h2 className="text-sm font-semibold">Deployment Guide</h2>
        </div>

        <div className="space-y-4">
          {/* Server Deployment */}
          <div className="border rounded-lg p-4 space-y-3">
            <h3 className="text-sm font-medium flex items-center gap-2">
              <Server className="h-4 w-4" />
              1. Deploy the server
            </h3>
            <div className="bg-muted/50 p-3 rounded-lg font-mono text-xs space-y-1">
              <p className="text-muted-foreground"># Build the server</p>
              <p>cd server &amp;&amp; go build -o ../multicode-server ./cmd/server</p>
              <p className="pt-2 text-muted-foreground"># Start the service</p>
              <p>export PORT=8080</p>
              <p>export DATABASE_URL=&quot;postgres://...&quot;</p>
              <p>./multicode-server</p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleCopyCommand(localCommands.deploy)}
            >
              <Copy className="h-4 w-4 mr-2" />
              Copy command
            </Button>
          </div>

          {/* Client Connection */}
          <div className="border rounded-lg p-4 space-y-3">
            <h3 className="text-sm font-medium flex items-center gap-2">
              <Globe className="h-4 w-4" />
              2. Connect a client
            </h3>
            <p className="text-xs text-muted-foreground">
              Run the following commands on another machine to connect to the
              server:
            </p>
            <div className="bg-muted/50 p-3 rounded-lg font-mono text-xs space-y-1">
              <p>export MULTICODE_SERVER_URL=&quot;{serverUrl}&quot;</p>
              <p>multicode login</p>
              <p>multicode daemon start</p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleCopyCommand(localCommands.connect)}
            >
              <Copy className="h-4 w-4 mr-2" />
              Copy command
            </Button>
          </div>

          {/* Security Note */}
          <div className="rounded-lg border border-warning/30 bg-warning/5 p-4">
            <div className="flex items-center gap-2 text-warning">
              <Lock className="h-4 w-4" />
              <span className="text-sm font-medium">Security Recommendations</span>
            </div>
            <ul className="mt-2 text-xs text-muted-foreground space-y-1">
              <li>Use HTTPS in production environments</li>
              <li>Configure an SSL reverse proxy with Nginx or Caddy</li>
              <li>Set firewall rules to restrict access</li>
              <li>Rotate API tokens regularly</li>
            </ul>
          </div>
        </div>
      </section>

      {/* Workspace Info */}
      {workspace && (
        <section className="space-y-4">
          <h2 className="text-sm font-semibold">Current Workspace</h2>
          <div className="bg-muted/50 rounded-lg p-4 space-y-2">
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">Name</span>
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
