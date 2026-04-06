import { useEffect, useState } from 'react'
import { getPermissions, onPermissionsChange } from '../services/api'

export const usePermissions = () => {
  const [permissions, setPermissions] = useState<string[]>(() => getPermissions())

  useEffect(() => {
    setPermissions(getPermissions())
    return onPermissionsChange(setPermissions)
  }, [])

  return permissions
}
