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
  "How do I start multiple Claude instances?",
  "How do I connect to a remote server?",
  "How do I configure a tunnel?",
  "What are the steps to create a new task?",
];

export function AIAssistant() {
  const [isOpen, setIsOpen] = useState(false);
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Message[]>([
    {
      role: "assistant",
      content: "Hi! I'm the Alphenix AI assistant. I can help with:\n\n• Starting and configuring agent instances\n• Connecting to remote servers\n• Setting up tunnels\n• Creating and managing tasks\n\nWhat can I help you with?",
    },
  ]);
  const [isLoading, setIsLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => { if (timerRef.current) clearTimeout(timerRef.current); }, []);

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

    // TODO: Replace with real AI API call
    timerRef.current = setTimeout(() => {
      const response =
        "This is a placeholder response. The AI assistant is not yet connected to a backend service.";
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: response },
      ]);
      setIsLoading(false);
    }, 1000);
  };

  return (
    <>
      {/* Floating Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        aria-label={isOpen ? "Close AI assistant" : "Open AI assistant"}
        aria-expanded={isOpen}
        className={cn(
          "fixed bottom-6 right-6 z-50 flex items-center justify-center rounded-full",
          "bg-brand text-brand-foreground",
          "shadow-lg dark:shadow-none hover:shadow-xl dark:hover:shadow-none transition-all duration-300",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          isOpen ? "size-10" : "size-14"
        )}
      >
        {isOpen ? (
          <X className="h-5 w-5" aria-hidden="true" />
        ) : (
          <Sparkles className="h-6 w-6" aria-hidden="true" />
        )}
      </button>

      {/* Chat Panel */}
      <div
        className={cn(
          "fixed bottom-24 right-6 z-50 w-96 rounded-xl",
          "bg-background border shadow-2xl dark:shadow-none",
          "transition-all duration-300 ease-out",
          "origin-bottom-right",
          isOpen
            ? "opacity-100 scale-100 translate-y-0"
            : "opacity-0 scale-95 translate-y-4 pointer-events-none"
        )}
        aria-hidden={!isOpen}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b bg-brand/5">
          <div className="flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-brand" aria-hidden="true" />
            <span className="font-semibold">Alphenix AI Assistant</span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => setIsOpen(false)}
            aria-label="Minimize AI assistant"
          >
            <ChevronDown className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>

        {/* Messages */}
        <div className="h-80 overflow-y-auto p-4 space-y-4" role="log" aria-live="polite" aria-label="Chat messages">
          {messages.map((message, index) => (
            <div
              key={`${message.role}-${index}`}
              className={cn(
                "flex",
                message.role === "user" ? "justify-end" : "justify-start"
              )}
            >
              <div
                className={cn(
                  "max-w-[85%] rounded-2xl px-4 py-2 text-sm break-words",
                  message.role === "user"
                    ? "bg-primary text-primary-foreground rounded-br-md"
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
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                <span className="sr-only">Assistant is typing...</span>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Suggestions */}
        {!messages.some((m) => m.role === "user") && (
          <div className="px-4 pb-2">
            <p className="text-xs text-muted-foreground mb-2">Try asking:</p>
            <div className="flex flex-wrap gap-1">
              {SUGGESTIONS.map((suggestion) => (
                <button
                  key={suggestion}
                  onClick={() => handleSuggestion(suggestion)}
                  className="text-xs bg-brand/10 text-brand px-2 py-1 rounded-full hover:bg-brand/20 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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
              placeholder="Ask a question..."
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
              aria-label="Send message"
            >
              <Send className="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        </form>
      </div>
    </>
  );
}
