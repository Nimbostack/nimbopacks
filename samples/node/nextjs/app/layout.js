export const metadata = {
  title: 'nimbopacks — Next.js sample',
  description: 'Next.js standalone output, served by node',
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
