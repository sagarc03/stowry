import { useState, useEffect, useCallback } from 'react'
import { FileUploader } from './components/FileUploader'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

interface FileRecord {
  id: number
  name: string
  key: string
  size: number
  content_type: string
  download_url: string
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function App() {
  const [files, setFiles] = useState<FileRecord[]>([])
  const [loading, setLoading] = useState(true)

  const fetchFiles = useCallback(async () => {
    try {
      const response = await fetch(`${API_URL}/api/files`)
      if (response.ok) {
        const data = await response.json()
        setFiles(data)
      }
    } catch (error) {
      console.error('Failed to fetch files:', error)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchFiles()
  }, [fetchFiles])

  const handleUploadComplete = () => {
    fetchFiles()
  }

  return (
    <div className="max-w-3xl mx-auto p-8">
      <h1 className="text-3xl font-bold mb-2">Flask + React SPA</h1>
      <p className="text-gray-600 mb-8">File upload example using Stowry with presigned URLs</p>

      <div className="bg-white rounded-lg p-6 mb-6 shadow-md">
        <h2 className="text-xl font-semibold mb-4">Upload Files</h2>
        <FileUploader onUploadComplete={handleUploadComplete} apiUrl={API_URL} />
      </div>

      <div className="bg-white rounded-lg p-6 shadow-md">
        <h2 className="text-xl font-semibold mb-4">Uploaded Files</h2>
        {loading ? (
          <p className="text-gray-500 italic">Loading files...</p>
        ) : files.length === 0 ? (
          <p className="text-gray-500 italic">No files uploaded yet.</p>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b">
                <th className="text-left py-3 px-2 font-semibold">Name</th>
                <th className="text-left py-3 px-2 font-semibold">Size</th>
                <th className="text-left py-3 px-2 font-semibold">Type</th>
                <th className="text-left py-3 px-2 font-semibold">Action</th>
              </tr>
            </thead>
            <tbody>
              {files.map((file) => (
                <tr key={file.id} className="border-b hover:bg-gray-50">
                  <td className="py-3 px-2">{file.name}</td>
                  <td className="py-3 px-2">{formatFileSize(file.size)}</td>
                  <td className="py-3 px-2">{file.content_type}</td>
                  <td className="py-3 px-2">
                    <a
                      href={file.download_url}
                      className="text-indigo-600 hover:underline"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Download
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

export default App
