import { StyleSheet } from 'react-native';

export const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8fafc',
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  loginCard: {
    margin: 20,
    backgroundColor: '#fff',
    borderRadius: 12,
    padding: 16,
    borderWidth: 1,
    borderColor: '#e2e8f0',
    gap: 10,
  },
  scroll: {
    padding: 16,
    paddingBottom: 100,
    gap: 12,
  },
  title: {
    fontSize: 22,
    fontWeight: '700',
    color: '#1d4ed8',
  },
  card: {
    backgroundColor: '#fff',
    borderRadius: 12,
    borderWidth: 1,
    borderColor: '#e2e8f0',
    padding: 14,
    gap: 8,
  },
  sectionTitle: {
    marginTop: 4,
    fontSize: 16,
    fontWeight: '700',
  },
  muted: {
    color: '#64748b',
  },
  error: {
    color: '#b91c1c',
  },
  success: {
    color: '#166534',
  },
  row: {
    flexDirection: 'row',
    gap: 8,
    flexWrap: 'wrap',
  },
  chip: {
    borderRadius: 999,
    paddingVertical: 6,
    paddingHorizontal: 12,
    backgroundColor: '#e2e8f0',
  },
  chipActive: {
    backgroundColor: '#2563eb',
  },
  chipDisabled: {
    opacity: 0.55,
  },
  chipText: {
    color: '#334155',
    fontSize: 12,
  },
  chipTextActive: {
    color: '#fff',
    fontSize: 12,
  },
  input: {
    borderWidth: 1,
    borderColor: '#cbd5e1',
    borderRadius: 10,
    padding: 10,
    backgroundColor: '#fff',
  },
  inputHalf: {
    minWidth: 140,
    flex: 1,
  },
  primaryBtn: {
    borderRadius: 10,
    backgroundColor: '#2563eb',
    paddingVertical: 12,
    alignItems: 'center',
  },
  primaryBtnText: {
    color: '#fff',
    fontWeight: '700',
  },
  secondaryBtn: {
    borderRadius: 10,
    backgroundColor: '#eef2ff',
    paddingVertical: 12,
    alignItems: 'center',
  },
  primaryBtnCompact: {
    borderRadius: 10,
    backgroundColor: '#2563eb',
    paddingVertical: 8,
    paddingHorizontal: 12,
    alignItems: 'center',
  },
  secondaryBtnCompact: {
    borderRadius: 10,
    backgroundColor: '#eef2ff',
    paddingVertical: 8,
    paddingHorizontal: 12,
    alignItems: 'center',
  },
  dangerBtnCompact: {
    borderRadius: 10,
    backgroundColor: '#fee2e2',
    paddingVertical: 8,
    paddingHorizontal: 12,
    alignItems: 'center',
  },
  secondaryBtnText: {
    color: '#3730a3',
    fontWeight: '600',
  },
  dangerBtnText: {
    color: '#991b1b',
    fontWeight: '600',
  },
  listItem: {
    paddingVertical: 8,
    borderBottomWidth: 1,
    borderBottomColor: '#e2e8f0',
    gap: 2,
  },
  tabBar: {
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 0,
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
    backgroundColor: '#fff',
    paddingVertical: 8,
  },
  tabBarContent: {
    flexDirection: 'row',
    paddingHorizontal: 8,
  },
  tabBtn: {
    minWidth: 68,
    alignItems: 'center',
    paddingVertical: 6,
  },
  tabText: {
    color: '#64748b',
  },
  tabActive: {
    color: '#1d4ed8',
    fontWeight: '700',
  },
});
