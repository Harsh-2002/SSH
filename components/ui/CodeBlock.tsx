import React, { useState } from 'react';
import { Copy, Check } from 'lucide-react';

interface CodeBlockProps {
  code: string;
  label?: string;
}

const CodeBlock: React.FC<CodeBlockProps> = ({ code, label }) => {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative font-mono text-sm group">
      {label && (
        <div className="absolute -top-3 left-4 bg-brand-yellow px-2 border-2 border-brand-black rounded text-xs font-bold z-10 shadow-[2px_2px_0px_0px_rgba(18,18,18,1)]">
          {label}
        </div>
      )}
      <div className="bg-brand-black text-gray-200 p-5 rounded-lg border-2 border-brand-black shadow-brutal overflow-x-auto transition-transform duration-300 hover:scale-[1.005]">
        <pre>
          <code>{code}</code>
        </pre>
      </div>
      <button 
        onClick={handleCopy}
        className="absolute top-4 right-4 p-2 bg-brand-yellow text-brand-black border-2 border-brand-black rounded hover:bg-white hover:-translate-y-[2px] hover:shadow-[2px_2px_0px_0px_rgba(18,18,18,1)] active:translate-y-0 active:shadow-none transition-all duration-200"
        title="Copy to clipboard"
      >
        {copied ? <Check size={16} /> : <Copy size={16} />}
      </button>
    </div>
  );
};

export default CodeBlock;