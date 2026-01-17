import React from 'react';

interface BrutalCardProps {
  children: React.ReactNode;
  title?: string;
  className?: string;
  noPadding?: boolean;
}

const BrutalCard: React.FC<BrutalCardProps> = ({ 
  children, 
  title, 
  className = '',
  noPadding = false
}) => {
  return (
    <div className={`bg-white border-2 border-brand-black rounded-xl shadow-brutal overflow-hidden ${className}`}>
      {title && (
        <div className="bg-brand-black text-white px-6 py-3 font-mono font-bold border-b-2 border-brand-black">
          {title}
        </div>
      )}
      <div className={noPadding ? '' : 'p-6'}>
        {children}
      </div>
    </div>
  );
};

export default BrutalCard;