





import { useQuery } from 'react-query'
import { admin } from '../api'
import { useForm } from 'react-hook-form'

export default function Secrets() {
  const { data: secrets, isLoading, refetch } = useQuery({
    queryKey: ['secrets'],
    queryFn: admin.secrets,
  })

  const { register, handleSubmit, formState: { errors } } = useForm({
    defaultValues: secrets?.data?.reduce((acc, secret) => {
      acc[secret.name] = secret.value
      return acc
    }, {})
  })

  const onSubmit = async (data) => {
    for (const [name, value] of Object.entries(data)) {
      await admin.saveSecret(name, value)
    }
    alert('Секреты сохранены!')
    refetch()
  }

  if (isLoading) {
    return <div className="flex justify-center items-center h-screen">Loading...</div>
  }

  return (
    <div className="max-w-7xl mx-auto p-8">
      <h1 className="text-4xl font-bold mb-8 dark:text-white">Секреты</h1>

      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
        <h2 className="text-2xl font-bold mb-6 dark:text-white">API ключи провайдеров</h2>

        <form onSubmit={handleSubmit(onSubmit)}>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            {secrets.data.map(secret => (
              <div key={secret.name} className="bg-gray-50 dark:bg-gray-700 p-6 rounded-xl">
                <h3 className="text-xl font-bold mb-4 dark:text-white">{secret.name}</h3>

                <div className="mb-4">
                  <label className="block text-gray-700 dark:text-gray-300 mb-2">Значение</label>
                  <input
                    type="text"
                    {...register(secret.name, {
                      required: 'Значение обязательно'
                    })}
                    className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-lg dark:bg-gray-600 dark:text-white"
                  />
                  {errors[secret.name] && (
                    <p className="text-red-500 text-sm mt-1">{errors[secret.name].message}</p>
                  )}
                </div>
              </div>
            ))}
          </div>

          <button
            type="submit"
            className="mt-8 bg-red-600 text-white px-8 py-4 rounded-lg hover:bg-red-700 transition"
          >
            Сохранить все секреты
          </button>
        </form>
      </div>
    </div>
  )
}





