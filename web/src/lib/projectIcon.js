export const projectIcons = [
  'mdi-server', 'mdi-cloud-outline', 'mdi-database', 'mdi-cube-outline',
  'mdi-network', 'mdi-router-network', 'mdi-shield-check-outline', 'mdi-code-braces',
  'mdi-rocket-launch-outline', 'mdi-cog-outline', 'mdi-flask-outline', 'mdi-home-outline',
  'mdi-web', 'mdi-lan', 'mdi-folder-outline', 'mdi-git',
  'mdi-package-variant-closed', 'mdi-desktop-tower-monitor', 'mdi-kubernetes', 'mdi-docker',
  'mdi-puzzle-outline', 'mdi-briefcase-outline', 'mdi-lightning-bolt-outline', 'mdi-star-outline',
];

export function projectInitials(name) {
  const parts = (name || '').trim().split(/\s+/).filter(Boolean);
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : (parts[0] || '').slice(0, 2)).toUpperCase();
}

export async function readProjectIcon(file) {
  if (!file || !['image/png', 'image/jpeg', 'image/webp'].includes(file.type)
    || file.size > 5 * 1024 * 1024) throw new Error('projectIconFormats');
  const url = URL.createObjectURL(file);
  try {
    const img = await new Promise((resolve, reject) => {
      const image = new Image();
      image.onload = () => resolve(image);
      image.onerror = () => reject(new Error('projectIconInvalid'));
      image.src = url;
    });
    if (!img.width || !img.height || img.width > 8192 || img.height > 8192) {
      throw new Error('projectIconInvalid');
    }
    const canvas = document.createElement('canvas');
    // A compact PNG works consistently in browsers and every database dialect.
    const sizes = [128, 96, 64];
    for (let i = 0; i < sizes.length; i += 1) {
      const size = sizes[i];
      canvas.width = size; canvas.height = size;
      const scale = Math.min(size / img.width, size / img.height);
      const width = img.width * scale; const height = img.height * scale;
      canvas.getContext('2d').drawImage(img, (size - width) / 2, (size - height) / 2, width, height);
      const data = canvas.toDataURL('image/png');
      if (data.length <= 60000) return data;
    }
    throw new Error('projectIconInvalid');
  } finally { URL.revokeObjectURL(url); }
}
