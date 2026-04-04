"use client";

import { useState, useRef, useEffect } from "react";
import { Sparkles, Send, X, ChevronDown, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

interface Message {
  role: "user" | "assistant";
  content: string;
}

const SUGGESTIONS = [
  "如何启动多个 Claude 实例？",
  "如何连接到远程服务器？",
  "如何配置内网穿透？",
  "创建新任务的步骤是什么？",
];

export function AIAssistant() {
  const [isOpen, setIsOpen] = useState(false);
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Message[]>([
    {
      role: "assistant",
      content: "你好！我是 Multicode AI 助手。我可以帮你：\n\n• 启动和配置多个 Claude 实例\n• 连接到远程服务器\n• 配置内网穿透\n• 创建和管理任务\n\n有什么我可以帮你的吗？",
    },
  ]);
  const [isLoading, setIsLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    if (isOpen) {
      scrollToBottom();
    }
  }, [messages, isOpen]);

  const handleSuggestion = (suggestion: string) => {
    setInput(suggestion);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;

    const userMessage = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: userMessage }]);
    setIsLoading(true);

    // Simulated AI response - in production, this would call the API
    setTimeout(() => {
      const responses = [
        "根据你的需求，我建议使用 `start-claude-cluster.sh` 脚本来启动多个实例。这个脚本会自动配置 MiniMax API 并管理端口。",
        "要连接到远程服务器，你需要在其他设备上运行 `connect-to-server.sh` 脚本，然后配置 `MULTICODE_SERVER_URL` 环境变量。",
        "对于内网穿透，我推荐使用 ngrok。你可以在服务器上运行 `ngrok http 8080` 来创建公网访问地址。",
        "创建任务的流程是：1) 在 Issues 页面创建 issue，2) 分配给 agent，3) agent 会自动接收并执行任务。",
      ];
      const randomResponse = responses[Math.floor(Math.random() * responses.length)]!;
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: randomResponse },
      ]);
      setIsLoading(false);
    }, 1000);
  };

  return (
    <>
      {/* Floating Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          "fixed bottom-6 right-6 z-50 flex items-center justify-center rounded-full",
          "bg-gradient-to-br from-purple-500 to-pink-500",
          "shadow-lg hover:shadow-xl transition-all duration-300",
          "w-14 h-14 text-white",
          isOpen ? "w-10 h-10 rounded-full" : "w-14 h-14 rounded-full"
        )}
        style={{
          boxShadow: "0 4px 20px rgba(168, 85, 247, 0.4)",
        }}
      >
        {isOpen ? (
          <X className="h-5 w-5" />
        ) : (
          <Sparkles className="h-6 w-6" />
        )}
      </button>

      {/* Chat Panel */}
      <div
        className={cn(
          "fixed bottom-24 right-6 z-50 w-96 rounded-xl",
          "bg-background border shadow-2xl",
          "transition-all duration-300 ease-out",
          "origin-bottom-right",
          isOpen
            ? "opacity-100 scale-100 translate-y-0"
            : "opacity-0 scale-95 translate-y-4 pointer-events-none"
        )}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b bg-gradient-to-r from-purple-500/10 to-pink-500/10">
          <div className="flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-purple-500" />
            <span className="font-semibold">Multicode AI 助手</span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => setIsOpen(false)}
          >
            <ChevronDown className="h-4 w-4" />
          </Button>
        </div>

        {/* Messages */}
        <div className="h-80 overflow-y-auto p-4 space-y-4">
          {messages.map((message, index) => (
            <div
              key={index}
              className={cn(
                "flex",
                message.role === "user" ? "justify-end" : "justify-start"
              )}
            >
              <div
                className={cn(
                  "max-w-[85%] rounded-2xl px-4 py-2 text-sm",
                  message.role === "user"
                    ? "bg-purple-500 text-white rounded-br-md"
                    : "bg-muted rounded-bl-md"
                )}
              >
                {message.content}
              </div>
            </div>
          ))}
          {isLoading && (
            <div className="flex justify-start">
              <div className="bg-muted rounded-2xl rounded-bl-md px-4 py-2">
                <Loader2 className="h-4 w-4 animate-spin" />
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Suggestions */}
        {!messages.some((m) => m.role === "user") && (
          <div className="px-4 pb-2">
            <p className="text-xs text-muted-foreground mb-2">试试问：</p>
            <div className="flex flex-wrap gap-1">
              {SUGGESTIONS.map((suggestion) => (
                <button
                  key={suggestion}
                  onClick={() => handleSuggestion(suggestion)}
                  className="text-xs bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 px-2 py-1 rounded-full hover:bg-purple-200 dark:hover:bg-purple-900/50 transition-colors"
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Input */}
        <form onSubmit={handleSubmit} className="p-4 border-t">
          <div className="flex gap-2">
            <Textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="输入你的问题..."
              className="min-h-[60px] resize-none"
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  handleSubmit(e);
                }
              }}
            />
            <Button
              type="submit"
              size="icon"
              disabled={!input.trim() || isLoading}
              className="shrink-0"
            >
              <Send className="h-4 w-4" />
            </Button>
          </div>
        </form>
      </div>
    </>
  );
}
