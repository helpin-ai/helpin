import { useState } from 'react';
import { Highlight, themes } from 'prism-react-renderer';
import { Copy, Check } from 'lucide-react';
import { Button } from './button';
import { toast } from 'sonner';
import { useTheme } from 'next-themes';

interface CodeBlockProps {
  code: string;
  language?: string;
  showLineNumbers?: boolean;
}

export function CodeBlock({ code, language = 'markup', showLineNumbers = false }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);
  const { resolvedTheme } = useTheme();

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    toast.success('Copied to clipboard');
    setTimeout(() => setCopied(false), 2000);
  };

  const theme = resolvedTheme === 'dark' ? themes.oneDark : themes.oneLight;

  return (
    <div className="relative group rounded-lg border overflow-hidden">
      <Highlight theme={theme} code={code.trim()} language={language}>
        {({ style, tokens, getLineProps, getTokenProps }) => (
          <pre
            className="p-4 text-xs overflow-x-auto !bg-muted/50"
            style={{ ...style, backgroundColor: undefined }}
          >
            {tokens.map((line, i) => (
              <div key={i} {...getLineProps({ line })}>
                {showLineNumbers && (
                  <span className="inline-block w-8 text-right mr-4 select-none text-muted-foreground/50">
                    {i + 1}
                  </span>
                )}
                {line.map((token, key) => (
                  <span key={key} {...getTokenProps({ token })} />
                ))}
              </div>
            ))}
          </pre>
        )}
      </Highlight>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="absolute top-2 right-2 h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity bg-background/80 backdrop-blur-sm"
        onClick={handleCopy}
      >
        {copied ? (
          <Check className="h-3.5 w-3.5 text-green-500" />
        ) : (
          <Copy className="h-3.5 w-3.5" />
        )}
      </Button>
    </div>
  );
}
