import { request, upload } from './client';
import { createApiClient } from './generated-client';

export const api = createApiClient({ request, upload });
