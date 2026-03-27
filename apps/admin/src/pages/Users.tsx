import { useEffect, useState, useCallback } from 'react'
import { adminApi, type User } from '../api'

export default function Users({ onNavigate }: { onNavigate: (path: string) => void }) {
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [query, setQuery] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const perPage = 20

  const load = useCallback(() => {
    adminApi.listUsers(page, query).then(data => {
      setUsers(data.users)
      setTotal(data.total)
    })
  }, [page, query])

  useEffect(() => { load() }, [load])

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setPage(1)
    setQuery(searchInput)
  }

  const totalPages = Math.ceil(total / perPage)

  return (
    <>
      <h3 className="section-title">Users ({total})</h3>

      <form className="search-bar" onSubmit={handleSearch}>
        <input
          type="text"
          className="input"
          placeholder="Search by email or name..."
          value={searchInput}
          onChange={e => setSearchInput(e.target.value)}
        />
      </form>

      <table>
        <thead>
          <tr>
            <th>Email</th>
            <th>Name</th>
            <th>Joined</th>
          </tr>
        </thead>
        <tbody>
          {users.map(u => (
            <tr key={u.ID}>
              <td>
                <a href={`/users/${u.ID}`} onClick={e => { e.preventDefault(); onNavigate(`/users/${u.ID}`) }}>
                  {u.Email}
                </a>
              </td>
              <td>{u.Name || <span style={{ color: 'var(--faint)' }}>—</span>}</td>
              <td>{new Date(u.CreatedAt).toLocaleDateString()}</td>
            </tr>
          ))}
          {users.length === 0 && (
            <tr><td colSpan={3} className="msg">No users found</td></tr>
          )}
        </tbody>
      </table>

      {totalPages > 1 && (
        <div className="pagination">
          <button disabled={page <= 1} onClick={() => setPage(p => p - 1)}>Prev</button>
          <span>Page {page} of {totalPages}</span>
          <button disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next</button>
        </div>
      )}
    </>
  )
}
