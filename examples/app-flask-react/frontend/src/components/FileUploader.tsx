import { useState, useRef } from 'react'

interface FileUploaderProps {
  onUploadComplete: () => void
  apiUrl: string
}

interface PresignResponse {
  url: string
  key: string
  filename: string
  content_type: string
}

export function FileUploader({ onUploadComplete, apiUrl }: FileUploaderProps) {
  const [uploading, setUploading] = useState(false)
  const [selectedFiles, setSelectedFiles] = useState<File[]>([])
  const [status, setStatus] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      setSelectedFiles(Array.from(e.target.files))
      setStatus(null)
    }
  }

  const handleUpload = async () => {
    if (selectedFiles.length === 0) {
      setStatus({ type: 'error', message: 'Please select files to upload' })
      return
    }

    setUploading(true)
    setStatus(null)

    try {
      for (const file of selectedFiles) {
        // 1. Get presigned URL from Flask backend
        const presignResponse = await fetch(`${apiUrl}/api/presign/upload`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            filename: file.name,
            content_type: file.type || 'application/octet-stream',
          }),
        })

        if (!presignResponse.ok) {
          throw new Error(`Failed to get presigned URL for ${file.name}`)
        }

        const presignData: PresignResponse = await presignResponse.json()

        // 2. Upload directly to Stowry using presigned URL
        const uploadResponse = await fetch(presignData.url, {
          method: 'PUT',
          body: file,
          headers: {
            'Content-Type': file.type || 'application/octet-stream',
          },
        })

        if (!uploadResponse.ok) {
          throw new Error(`Failed to upload ${file.name}`)
        }

        // 3. Save file metadata to Flask backend
        await fetch(`${apiUrl}/api/files`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: file.name,
            key: presignData.key,
            size: file.size,
            content_type: file.type || 'application/octet-stream',
          }),
        })
      }

      setStatus({
        type: 'success',
        message: `Successfully uploaded ${selectedFiles.length} file(s)`,
      })

      // Clear selection
      setSelectedFiles([])
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }

      // Refresh file list
      onUploadComplete()
    } catch (error) {
      setStatus({
        type: 'error',
        message: error instanceof Error ? error.message : 'Upload failed',
      })
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="space-y-4">
      <label className="block cursor-pointer">
        <div className="border-2 border-dashed border-gray-300 rounded-lg p-8 text-center hover:border-indigo-500 hover:bg-indigo-50 transition-colors">
          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            disabled={uploading}
            onChange={handleFileChange}
          />
          <svg className="mx-auto h-12 w-12 text-gray-400" stroke="currentColor" fill="none" viewBox="0 0 48 48">
            <path d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <p className="mt-4 text-lg">
            <span className="font-semibold text-indigo-600">Click to select files</span>
          </p>
          <p className="mt-1 text-sm text-gray-500">or drag and drop</p>
        </div>
      </label>

      {selectedFiles.length > 0 && (
        <div className="bg-gray-50 rounded-lg p-4">
          <p className="text-sm font-medium text-gray-700 mb-2">
            Selected ({selectedFiles.length}):
          </p>
          <ul className="text-sm text-gray-600 space-y-1">
            {selectedFiles.map((file, i) => (
              <li key={i}>{file.name} ({(file.size / 1024).toFixed(1)} KB)</li>
            ))}
          </ul>
        </div>
      )}

      <button
        onClick={handleUpload}
        disabled={uploading || selectedFiles.length === 0}
        className="w-full bg-indigo-600 text-white px-6 py-3 rounded-lg font-medium hover:bg-indigo-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
      >
        {uploading ? 'Uploading...' : 'Upload'}
      </button>

      {status && (
        <div
          className={`p-3 rounded-lg ${
            status.type === 'success'
              ? 'bg-green-100 text-green-800'
              : 'bg-red-100 text-red-800'
          }`}
        >
          {status.message}
        </div>
      )}
    </div>
  )
}
