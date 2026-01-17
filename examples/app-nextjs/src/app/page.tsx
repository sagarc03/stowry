import { getFiles, type FileRecord } from "@/lib/db";
import { getDownloadUrl } from "@/lib/stowry";
import { UploadForm } from "@/components/upload-form";

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

async function getFilesWithUrls(): Promise<(FileRecord & { downloadUrl: string })[]> {
  const files = getFiles();
  return Promise.all(
    files.map(async (file) => ({
      ...file,
      downloadUrl: await getDownloadUrl(file.key),
    }))
  );
}

export default async function Home() {
  const files = await getFilesWithUrls();

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-50 font-sans dark:bg-zinc-950">
      <main className="flex w-full max-w-2xl flex-col items-center gap-8 p-8">
        <div className="text-center">
          <h1 className="text-3xl font-bold text-zinc-900 dark:text-zinc-100">
            Stowry + Better Upload
          </h1>
          <p className="mt-2 text-zinc-600 dark:text-zinc-400">
            Upload files directly to Stowry using Better Upload
          </p>
        </div>

        <UploadForm />

        {files.length > 0 && (
          <div className="w-full">
            <h2 className="mb-4 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
              Uploaded Files
            </h2>
            <div className="overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-800">
              <table className="w-full">
                <thead className="bg-zinc-100 dark:bg-zinc-900">
                  <tr>
                    <th className="px-4 py-3 text-left text-sm font-medium text-zinc-600 dark:text-zinc-400">
                      Name
                    </th>
                    <th className="px-4 py-3 text-left text-sm font-medium text-zinc-600 dark:text-zinc-400">
                      Size
                    </th>
                    <th className="px-4 py-3 text-right text-sm font-medium text-zinc-600 dark:text-zinc-400">
                      Action
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-200 dark:divide-zinc-800">
                  {files.map((file) => (
                    <tr
                      key={file.id}
                      className="bg-white dark:bg-zinc-950 hover:bg-zinc-50 dark:hover:bg-zinc-900"
                    >
                      <td className="px-4 py-3">
                        <p className="truncate max-w-[200px] font-medium text-zinc-900 dark:text-zinc-100">
                          {file.name}
                        </p>
                        <p className="text-xs text-zinc-500">{file.content_type}</p>
                      </td>
                      <td className="px-4 py-3 text-sm text-zinc-600 dark:text-zinc-400">
                        {formatBytes(file.size)}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <a
                          href={file.downloadUrl}
                          className="inline-flex items-center gap-1 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
                          download={file.name}
                        >
                          Download
                        </a>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
