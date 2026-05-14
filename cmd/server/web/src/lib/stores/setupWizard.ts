import type { EnvParams, StorageParams } from '$lib/api'
import { writable } from 'svelte/store'

export interface WizardState {
  storage: StorageParams | null
  env: EnvParams | null
}

export const defaultStorageParams = (): StorageParams => ({
  type: 'local',
  localPath: '',
  s3Bucket: '',
  s3Region: '',
  s3Endpoint: '',
  s3AccessKey: '',
  s3SecretKey: '',
  sftpHost: '',
  sftpPort: 22,
  sftpUser: '',
  sftpKeyPath: '',
  sftpRemotePath: '',
})

export const defaultEnvParams = (): EnvParams => ({
  ...defaultStorageParams(),
  type: 'local',
  port: 14000,
  baseURL: '',
  smtpHost: '',
  smtpPort: 587,
  smtpUser: '',
  smtpPass: '',
  oidcIssuer: '',
  oidcClientID: '',
  oidcClientSecret: '',
})

export const wizardStore = writable<WizardState>({
  storage: null,
  env: null,
})
