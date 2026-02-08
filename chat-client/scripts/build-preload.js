const { execSync } = require('child_process');
const path = require('path');

console.log('Building preload script...');
try {
  execSync('npx tsc src/preload.ts --outDir dist --module commonjs --target es2020 --moduleResolution node --esModuleInterop --skipLibCheck', {
    stdio: 'inherit',
    cwd: path.join(__dirname, '..'),
  });
  console.log('Preload script built successfully!');
} catch (error) {
  console.error('Failed to build preload script:', error);
  process.exit(1);
}
