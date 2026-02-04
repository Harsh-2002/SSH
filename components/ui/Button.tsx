import React from 'react';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline';
  fullWidth?: boolean;
}

const Button: React.FC<ButtonProps> = ({
  children,
  variant = 'primary',
  fullWidth = false,
  className = '',
  ...props
}) => {
  const baseStyles = "font-bold text-sm md:text-base py-3 px-6 rounded-xl border-2 border-brand-black transition-all duration-200 active:translate-x-[2px] active:translate-y-[2px] active:shadow-none hover:-translate-y-[2px] hover:-translate-x-[2px] hover:shadow-[6px_6px_0px_0px_rgba(18,18,18,1)] font-mono";

  const variants = {
    primary: "bg-brand-yellow text-brand-black shadow-brutal hover:bg-yellow-300",
    secondary: "bg-brand-black text-brand-white shadow-brutal hover:bg-neutral-800",
    outline: "bg-white text-brand-black shadow-brutal hover:bg-gray-50"
  };

  const widthClass = fullWidth ? "w-full" : "";

  return (
    <button
      className={`${baseStyles} ${variants[variant]} ${widthClass} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
};

export default Button;